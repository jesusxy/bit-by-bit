package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
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

	eventChannel := make(chan model.Event, 1000) // channel that holds 100 events
	alertChannel := make(chan model.Alert, 500)
	stateManager := rules.NewStateManager()

	// start background services
	go startMetricsServer(ctx)
	go ingester.TailFile("/var/log/audit/audit.log", eventChannel)
	go handleAlerts(alertChannel)

	slog.Info("Nox IDS engine started",
		"version", "0.1.0",
		"log_file", "/var/log/audit/audit.log",
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
				processEvent(event, db, stateManager, alertChannel)
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

func processEvent(event model.Event, db *geoip2.Reader, stateManager *rules.StateManager, alertChannel chan<- model.Alert) {
	eventsProcessedTotal.Inc()

	triggeredAlerts := rules.EvaluateEvent(event, stateManager)

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

func handleAlerts(alertChan <-chan model.Alert) {}

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
