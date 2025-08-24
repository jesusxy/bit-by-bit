package main

import (
	"fmt"
	"log"
	"nox/internal/ingester"
	"nox/internal/model"
	"nox/internal/rules"
)

func main() {
	eventChannel := make(chan model.Event, 100) // channel that holds 100 events
	stateManager := rules.NewStateManager()

	go ingester.TailFile("testdata/auth.log", eventChannel)
	log.Println("Nox IDS engine started. Tailing log file...")

	for event := range eventChannel {
		triggeredAlerts := rules.EvaluateEvent(event, stateManager)

		for _, alert := range triggeredAlerts {
			fmt.Printf("ALERT: [%s] %s\n", alert.RuleName, alert.Message)
		}
	}
}
