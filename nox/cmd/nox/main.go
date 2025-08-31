package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"nox/internal/ingester"
	"nox/internal/model"
	"nox/internal/rules"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oschwald/geoip2-golang"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	db, err := geoip2.Open("testdata/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatalf("Error opening GeoIP database: %v", err)
	}
	defer db.Close()

	// can I instead use signal.NotifyContext() to simplify signal handling?
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	yamlRules, err := rules.LoadRulesFromFile("rules.yaml")
	if err != nil {
		log.Fatalf("could not load detection rules: %v", err)
	}

	eventChannel := make(chan model.Event, 1000) // channel that holds 100 events
	alertChannel := make(chan model.Alert, 500)
	stateManager := rules.NewStateManager()

	// start background services
	go startMetricsServer(ctx)
	go ingester.TailFile("testdata/auth.log", eventChannel)
	go handleAlerts(alertChannel)

	slog.Info("Nox IDS engine started",
		"version", "0.1.0",
		"log_file", "testdata/auth.log",
		"buffer_size", 1000,
	)

	// set up graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// main processing loop
	go func() {
		defer close(alertChannel)

		for {
			select {
			case event, ok := <-eventChannel:
				if !ok {
					slog.Info("Event channel closed, shutting down event processor")
					return
				}
				processEvent(event, yamlRules, db, stateManager, alertChannel)
			case <-ctx.Done():
				slog.Info("Context cancelled, shutting down event processor")
				return
			}
		}
	}()

	<-signalChan
	slog.Info("Shutdown signal received, gracefull stopping...")
	cancel()

	time.Sleep(2 * time.Second)
	slog.Info("Nox IDS engine stopped")
}

func processEvent(event model.Event, yamlRules []rules.RuleDefinition, db *geoip2.Reader, stateManager *rules.StateManager, alertChannel chan<- model.Alert) {
	eventsProcessedTotal.Inc()

	if event.Source != "localhost" && event.Source != "" {
		ip := net.ParseIP(event.Source)
		if ip != nil && !ip.IsPrivate() {
			if record, err := db.City(ip); err == nil {
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

	slog.Debug("Processing Event",
		"type", event.EventType,
		"source", event.Source,
		"timestamp", event.Timestamp,
	)

	triggeredAlerts := rules.EvaluateEvent(event, yamlRules, stateManager)

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

func handleAlerts(alertChan <-chan model.Alert) {
	for alert := range alertChan {
		alertsTriggeredTotal.WithLabelValues(alert.RuleName).Inc()
		alertsBySeverityTotal.WithLabelValues(alert.Severity).Inc()

		logLevel := slog.LevelWarn

		if alert.IsHighPriority() {
			logLevel = slog.LevelError
		}

		logger := slog.With(
			"alert_id", generateAlertID(alert),
			"rule_name", alert.RuleName,
			"severity", logLevel,
			"source", alert.Source,
			"timestamp", alert.Timestamp,
		)

		for key, value := range alert.Metadata {
			logger = logger.With(key, value)
		}

		logger.Log(context.Background(), logLevel, alert.Message)

	}
}

func generateAlertID(alert model.Alert) string {
	return fmt.Sprintf("%s-%d-%s",
		alert.RuleName,
		alert.Timestamp.Unix(),
		alert.Source,
	)
}

func startMetricsServer(ctx context.Context) {
	// set up server config
	server := &http.Server{
		Addr:    ":9090",
		Handler: promhttp.Handler(),
	}

	go func() {
		slog.Info("Starting Metrics server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Metrics server error", "error", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Metrics server shutdown error", "error", err)
	} else {
		slog.Info("Metrics server stopped")
	}
}
