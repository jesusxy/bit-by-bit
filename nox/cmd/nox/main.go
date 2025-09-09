package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"nox/internal/ingester"
	"nox/internal/model"
	"nox/internal/rules"
	"nox/internal/server"
	"nox/internal/storage"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	pb "nox/proto"
)

var (
	eventsProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "nox_events_processed_total",
		Help: "Total number of events processed by the engine.",
	})

	alertsTriggeredTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nox_alerts_triggered_total",
		Help: "Total number of alerts triggered.",
	}, []string{"rule_name"})

	alertsBySeverityTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "nox_alerts_by_severity_total",
		Help: "Total number of alerts by severity",
	}, []string{"severity"})

	// eventsByTypeTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	// 	Name: "nox_events_by_type_total",
	// 	Help: "Total number of events by type",
	// }, []string{"event_type"})

	// eventProcessingDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	// 	Name:    "nox_even_processing_duration_seconds",
	// 	Help:    "Time spent processing events",
	// 	Buckets: prometheus.DefBuckets,
	// }, []string{"event_type"})

	// geoIpLookupsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
	// 	Name: "nox_goeip_lookups_total",
	// 	Help: "Total number of geo lookups",
	// }, []string{"status"}) // success, failure, skipped

	// alertChannelSize = prometheus.NewGauge(prometheus.GaugeOpts{
	// 	Name: "nox_alert_channel_size",
	// 	Help: "Current size of alert channel buffer",
	// })

	// eventChannelSize = prometheus.NewGauge(prometheus.GaugeOpts{
	// 	Name: "nox_event_channel_size",
	// 	Help: "Current size of event channel buffer",
	// })

	// activeFailedLoginAttempts = prometheus.NewGauge(prometheus.GaugeOpts{
	// 	Name: "nox_active_failed_login_attempts",
	// 	Help: "Current number of active failed login attempt trackers",
	// })

	// activeUserLoginLocations = prometheus.NewGauge(prometheus.GaugeOpts{
	// 	Name: "nox_active_user_login_locations",
	// 	Help: "Current number of tracked user login locations",
	// })
)

func init() {
	prometheus.MustRegister(eventsProcessedTotal)
	prometheus.MustRegister(alertsTriggeredTotal)
	prometheus.MustRegister(alertsBySeverityTotal)
}

type ESConfig struct {
	URL string
}

type GRPCConfig struct {
	Addr string
}

type MetricsConfig struct {
	Addr string
}
type Config struct {
	RulesPath     string
	IntelPath     string
	LogPath       string
	GeoIPDBPath   string
	Elasticsearch ESConfig
	GRPC          GRPCConfig
	Metrics       MetricsConfig
	BufferSize    int
}

type Nox struct {
	Config   *Config
	Logger   *slog.Logger
	ESClient *storage.ESClient
	GeoIPDB  *geoip2.Reader
	Rules    []rules.RuleDefinition
	StateMgr *rules.StateManager
	wg       sync.WaitGroup
}

func NewNox(cfg *Config, logger *slog.Logger) (*Nox, error) {
	db, err := geoip2.Open(cfg.GeoIPDBPath)
	if err != nil {
		return nil, fmt.Errorf("error opening GeoIP database: %w", err)
	}

	esClient, err := storage.NewESClient(cfg.Elasticsearch.URL)
	if err != nil {
		db.Close() // why do we close here instead of doing defer db.close() outside of this block
		return nil, fmt.Errorf("could not crete Elasticsearch client: %w", err)
	}

	yamlRules, err := rules.LoadRulesFromFile(cfg.RulesPath)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("could not load detection rules: %w", err)
	}

	stateManager := rules.NewStateManager()
	if err := rules.LoadIPWatchlistFromFile(cfg.IntelPath, stateManager); err != nil {
		logger.Error("failed to load IP watchlist", "error", err)
	}

	return &Nox{
		Config:   cfg,
		Logger:   logger,
		ESClient: esClient,
		GeoIPDB:  db,
		Rules:    yamlRules,
		StateMgr: stateManager,
	}, nil

}

// Run starts all background services and the main processing loop.
func (n *Nox) Run(ctx context.Context) error {
	// ---- Ensure Elasticsearch Indices exists ---
	n.Logger.Info("Waiting for Elasticsearch...")
	time.Sleep(10 * time.Second)
	n.ESClient.EnsureIndex(ctx, "process_executed")
	n.ESClient.EnsureIndex(ctx, "sshd_accepted_password")
	n.ESClient.EnsureIndex(ctx, "sshd_failed_password")
	n.Logger.Info("Elasticsearch indices are ready.")

	eventChannel := make(chan model.Event, n.Config.BufferSize)
	alertChannel := make(chan model.Alert, n.Config.BufferSize/2)

	// --- Start Background Services ---
	n.startMetricsServer(ctx)
	n.startGRPCServer(ctx)
	n.startFileIngester(ctx, eventChannel)
	n.startAlertHandler(ctx, alertChannel)

	n.Logger.Info("Nox IDS engine started",
		"version", "0.1.0",
		"log_file", n.Config.LogPath,
		"buffer_size", n.Config.BufferSize,
	)

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		defer close(alertChannel)
		n.Logger.Info("Event processor started.")

		for {
			select {
			case event, ok := <-eventChannel:
				if !ok {
					n.Logger.Info("Event channel close, stopping event processor.")
					return
				}

				n.processEvent(event, alertChannel)
			case <-ctx.Done():
				n.Logger.Info("Context cancelled, stopping event processor")
				return
			}
		}
	}()

	n.wg.Wait()
	return nil
}

func (n *Nox) Stop() {
	n.Logger.Info("Stopping Nox services")
	if n.GeoIPDB != nil {
		n.GeoIPDB.Close()
	}

	n.Logger.Info("Shutdown complete..")
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	cfg := &Config{
		RulesPath:   getEnv("NOX_RULES_PATH", "detections/rules.yaml"),
		IntelPath:   getEnv("NOX_INTEL_PATH", "intel/ip_watchlist.txt"),
		LogPath:     "testdata/auth.log",
		GeoIPDBPath: "testdata/GeoLite2-City.mmdb",
		Elasticsearch: ESConfig{
			URL: "http://elasticsearch:9200",
		},
		GRPC: GRPCConfig{
			Addr: ":50051",
		},
		Metrics: MetricsConfig{
			Addr: ":9090",
		},
		BufferSize: 1000,
	}

	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		logger.Info("Shutdown signal received, gracefully stopping...")
		cancel()
	}()

	// --- CREATE AND RUN ENGINE ---
	nox, err := NewNox(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize application", "error", err)
		os.Exit(1)
	}

	if err := nox.Run(ctx); err != nil {
		logger.Error("Application runtime error", "error", err)
		os.Exit(1)
	}

	nox.Stop()
	slog.Info("Nox IDS engine stopped")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func generateAlertID(alert model.Alert) string {
	return fmt.Sprintf("%s-%d-%s",
		alert.RuleName,
		alert.Timestamp.Unix(),
		alert.Source,
	)
}

// --- Nox Methods (Engine Logic) ---
func (n *Nox) processEvent(event model.Event, alertChannel chan<- model.Alert) {
	eventsProcessedTotal.Inc()

	go n.ESClient.IndexEvent(context.Background(), event)

	if event.Source != "localhost" && event.Source != "" {
		ip := net.ParseIP(event.Source)
		if ip != nil && !ip.IsPrivate() {
			if record, err := n.GeoIPDB.City(ip); err == nil {
				if event.Metadata == nil {
					event.Metadata = make(map[string]string)
				}

				// add geographic metadata
				event.Metadata["country"] = record.Country.IsoCode
				event.Metadata["country_name"] = record.Country.Names["en"]
				if len(record.City.Names) > 0 {
					event.Metadata["city"] = record.City.Names["en"]
				}

				// add coordinates for geographic analysis
				if record.Location.Latitude != 0 && record.Location.Longitude != 0 {
					event.Metadata["latitude"] = fmt.Sprintf("%.4f", record.Location.Latitude)
					event.Metadata["longitude"] = fmt.Sprintf("%.4f", record.Location.Longitude)
				}

			} else {
				slog.Debug("GeoIP lookup failed", "ip", event.Source, "error", err)
			}
		}
	}

	n.Logger.Debug("Processing Event",
		"type", event.EventType,
		"source", event.Source,
		"timestamp", event.Timestamp,
	)

	triggeredAlerts := rules.EvaluateEvent(event, n.Rules, n.StateMgr)
	correlationAlerts := rules.RunCorrelationRules(event, triggeredAlerts, n.StateMgr)

	if len(correlationAlerts) > 0 {
		triggeredAlerts = append(triggeredAlerts, correlationAlerts...)
	}

	for _, alert := range triggeredAlerts {
		select {
		case alertChannel <- alert:
			// alert sent successfully
		default:
			slog.Warn("Alert channel full, dropping alert",
				"rule_name", alert.RuleName,
				"source", alert.Source,
			)
		}
	}
}

func (n *Nox) startAlertHandler(ctx context.Context, alertChan <-chan model.Alert) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.Logger.Info("Alert handler started.")
		for {
			select {
			case alert, ok := <-alertChan:
				if !ok {
					n.Logger.Info("Alert channel closed, stopping alert handler.")
					return
				}
				alertsTriggeredTotal.WithLabelValues(alert.RuleName).Inc()
				alertsBySeverityTotal.WithLabelValues(alert.Severity).Inc()

				logLevel := slog.LevelWarn
				if alert.IsHighPriority() {
					logLevel = slog.LevelError
				}

				logger := n.Logger.With(
					"alert_id", generateAlertID(alert),
					"rule_name", alert.RuleName,
					"severity", alert.Severity,
					"source", alert.Source,
					"timestamp", alert.Timestamp,
				)

				for key, value := range alert.Metadata {
					logger = logger.With(key, value)
				}

				logger.Log(ctx, logLevel, alert.Message)

			case <-ctx.Done():
				n.Logger.Info("Context cancelled, stopping alert handler.")
				return
			}
		}
	}()
}

func (n *Nox) startMetricsServer(ctx context.Context) {
	// set up server config
	server := &http.Server{
		Addr:    n.Config.Metrics.Addr,
		Handler: promhttp.Handler(),
	}
	n.wg.Add(1)

	go func() {
		defer n.wg.Done()
		n.Logger.Info("Starting Metrics server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server error", "error", err)
		}
		n.Logger.Info("Metrics server stopped")
	}()

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Metrics server shutdown error", "error", err)
		}
	}()
}

func (n *Nox) startGRPCServer(ctx context.Context) {
	lis, err := net.Listen("tcp", n.Config.GRPC.Addr)
	if err != nil {
		slog.Error("Failed to listen for gRPC", "error", err)
		return
	}

	s := grpc.NewServer()
	apiServer := server.NewNoxAPIServer(n.ESClient)
	pb.RegisterNoxServiceServer(s, apiServer)

	n.wg.Add(1)

	go func() {
		defer n.wg.Done()
		slog.Info("gRPC server started", "address", lis.Addr().String())
		// s.Serve is a blocking call, it will run until the server is stopped.
		if err := s.Serve(lis); err != nil {
			slog.Error("gRPC server failed", "error", err)
		}
		n.Logger.Info("gRPC server stopped.")
	}()

	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		<-ctx.Done()
		s.GracefulStop()
	}()
}

func (n *Nox) startFileIngester(ctx context.Context, eventChannel chan<- model.Event) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ingester.TailFile(n.Config.LogPath, eventChannel)
		close(eventChannel)
	}()
}
