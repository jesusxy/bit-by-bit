package main

import (
	"fmt"
	"log"
	"nox/internal/ingester"
	"nox/internal/rules"
)

func main() {
	stateManager := rules.NewStateManager()

	events, err := ingester.ReadFile("testdata/auth.log") // how do i create fake logs or is there an api i can use?
	if err != nil {
		log.Printf("failed to read file %v\n", err)
	}

	for _, event := range events {
		triggeredAlerts := rules.EvaluateEvent(event, stateManager)

		for _, alert := range triggeredAlerts {
			fmt.Printf("ALERT: [%s] %s\n", alert.RuleName, alert.Message)
		}
	}
}
