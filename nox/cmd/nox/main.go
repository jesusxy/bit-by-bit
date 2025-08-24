package main

import (
	"fmt"
	"log"
	"net"
	"nox/internal/ingester"
	"nox/internal/model"
	"nox/internal/rules"

	"github.com/oschwald/geoip2-golang"
)

func main() {
	db, err := geoip2.Open("testdata/GeoLite2-City.mmdb")
	if err != nil {
		log.Fatalf("Error opening GeoIP database: %v", err)
	}
	defer db.Close()

	eventChannel := make(chan model.Event, 100) // channel that holds 100 events
	stateManager := rules.NewStateManager()

	go ingester.TailFile("testdata/auth.log", eventChannel)
	log.Println("Nox IDS engine started. Tailing log file...")

	for event := range eventChannel {
		ip := net.ParseIP(event.Source)
		if ip != nil {
			record, err := db.Country(ip)
			if err == nil && record.Country.IsoCode != "" {
				event.Metadata["country"] = record.Country.IsoCode // e.g "US", "CN", "DE"
			}
		}

		triggeredAlerts := rules.EvaluateEvent(event, stateManager)

		for _, alert := range triggeredAlerts {
			fmt.Printf("ALERT: [%s] %s\n", alert.RuleName, alert.Message)
		}
	}
}
