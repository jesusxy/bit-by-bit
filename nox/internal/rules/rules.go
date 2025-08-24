package rules

import (
	"fmt"
	"nox/internal/model"
	"sync"
	"time"
)

type Rule func(event model.Event, state *StateManager) *model.Alert

type StateManager struct {
	mu sync.RWMutex
	// Tracks failed login attempts. Key: IP Address, Value: List of timestamps.
	FailedLoginAttempts map[string][]time.Time
	// we can add more state maps here for future rules, e.g.:
	// UserLoginLocations map[string]map[string]bool // Key: Username, Value: Set of countries
}

func NewStateManager() *StateManager {
	return &StateManager{
		FailedLoginAttempts: make(map[string][]time.Time),
	}
}

func checkFailedLogins(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "SSHD_Failed_Password" {
		return nil
	}

	//--- state logic ----//
	ip := event.Source
	const (
		threshold = 5
		window    = 60 * time.Second
	)

	attempts := state.FailedLoginAttempts[ip]
	attempts = append(attempts, event.Timestamp)

	// prune old timestamps that are outside the time window
	var recentAttempts []time.Time
	cutoff := time.Now().UTC().Add(-window)
	for _, t := range attempts {
		if t.After(cutoff) {
			recentAttempts = append(recentAttempts, t)
		}
	}

	state.FailedLoginAttempts[ip] = recentAttempts

	if len(recentAttempts) >= threshold {
		return &model.Alert{
			RuleName: "TooManyFailedLogins",
			Message:  fmt.Sprintf("Detected %d failed SSH logins from %s in the last minute.", len(recentAttempts), ip),
		}
	}

	return nil // no alert
}

// activeRules is the registry of all rules the engine will run.
var activeRules = []Rule{
	checkFailedLogins,
}

func EvaluateEvent(evt model.Event, state *StateManager) []model.Alert {
	var triggeredAlerts []model.Alert

	for _, rule := range activeRules {
		if alert := rule(evt, state); alert != nil {
			triggeredAlerts = append(triggeredAlerts, *alert)
		}
	}

	return triggeredAlerts
}
