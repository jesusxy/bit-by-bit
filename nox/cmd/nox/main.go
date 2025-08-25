package main

import (
	"log"
	"log/slog"
	"net"
	"net/http"
	"nox/internal/ingester"
	"nox/internal/model"
	"nox/internal/rules"

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

func startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Metrics server starting on port :9090")
	if err := http.ListenAndServe(":9090", nil); err != nil {
		slog.Error("Metrics server failed to start", "error", err)
	}
}

func main() {
	db, err := geoip2.Open("testdata/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatalf("Error opening GeoIP database: %v", err)
	}
	defer db.Close()

	eventChannel := make(chan model.Event, 100) // channel that holds 100 events
	stateManager := rules.NewStateManager()

	go startMetricsServer()
	go ingester.TailFile("testdata/auth.log", eventChannel)
	slog.Info("Nox IDS engine started", "version", "0.1.0")

	for event := range eventChannel {
		eventsProcessedTotal.Inc()
		ip := net.ParseIP(event.Source)
		if ip != nil {
			record, err := db.Country(ip)
			if err == nil && record.Country.IsoCode != "" {
				event.Metadata["country"] = record.Country.IsoCode // e.g "US", "CN", "DE"
			}
		}

		triggeredAlerts := rules.EvaluateEvent(event, stateManager)

		for _, alert := range triggeredAlerts {
			alertsTriggeredTotal.WithLabelValues(alert.RuleName).Inc()
			slog.Warn("Alert triggered",
				"rule_name", alert.RuleName,
				"message", alert.Message,
				"source_ip", event.Source,
			)
		}
	}
}
