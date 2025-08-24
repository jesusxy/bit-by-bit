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
	UserLoginLocations map[string]map[string]bool // Key: Username, Value: Set of country codes
}

func NewStateManager() *StateManager {
	return &StateManager{
		FailedLoginAttempts: make(map[string][]time.Time),
		UserLoginLocations:  make(map[string]map[string]bool),
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

func checkNewCountryLogins(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "SSHD_Accepted_Password" {
		return nil
	}

	user := event.Metadata["user"]
	country, ok := event.Metadata["country"]
	if !ok || user == "" {
		return nil // cant evaluate without user and country
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// check if user has a record of locations
	if _, ok := state.UserLoginLocations[user]; !ok {
		state.UserLoginLocations[user] = make(map[string]bool)
	}

	// check if country is new for user
	if !state.UserLoginLocations[user][country] {
		// new location, trigger alert
		state.UserLoginLocations[user][country] = true
		return &model.Alert{
			RuleName: "NewCountryLogin",
			Message:  fmt.Sprintf("User '%s' logged in from a new country: %s (IP: %s)", user, country, event.Source),
		}
	}

	return nil
}

// activeRules is the registry of all rules the engine will run.
var activeRules = []Rule{
	checkFailedLogins,
	checkNewCountryLogins,
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
