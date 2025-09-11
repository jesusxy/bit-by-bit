package rules

import (
	"fmt"
	"nox/internal/model"
	"time"
)

// --- Failed Logins Rule ----

type FailedLoginsRule struct {
	Threshold int
	Window    time.Duration
}

func NewFailedLoginsRule() Rule {
	return &FailedLoginsRule{
		Threshold: 5,
		Window:    60 * time.Second,
	}
}

func (r *FailedLoginsRule) Name() string {
	return "TooManyFailedLogins"
}

func (r *FailedLoginsRule) Evaluate(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "SSHD_Failed_Password" {
		return nil
	}

	stateMgr := state.FailedLogins
	stateMgr.mu.Lock()
	defer stateMgr.mu.Unlock()

	ip := event.Source

	if stateMgr.AlertedIPs[ip] {
		return nil // already alerted for this IP
	}

	var recentAttempts []time.Time
	now := time.Now().UTC()
	for _, t := range stateMgr.Attempts[ip] {
		if now.Sub(t) <= r.Window {
			recentAttempts = append(recentAttempts, t)
		}
	}

	recentAttempts = append(recentAttempts, event.Timestamp)
	stateMgr.Attempts[ip] = recentAttempts

	if len(recentAttempts) >= r.Threshold {
		stateMgr.AlertedIPs[ip] = true

		return &model.Alert{
			RuleName:  r.Name(),
			Message:   fmt.Sprintf("Detected %d failed SSH logins from %s in the last minute.", len(recentAttempts), ip),
			Severity:  "HIGH",
			Timestamp: event.Timestamp,
			Source:    ip,
			Metadata: map[string]string{
				"attempt_count": fmt.Sprintf("%d", len(recentAttempts)),
				"time_window":   r.Window.String(),
			},
		}
	}

	return nil
}

// ---- New Country Logins ----

type LoginLocationRule struct{}

func NewLoginLocationRule() Rule {
	return &LoginLocationRule{}
}

func (r *LoginLocationRule) Name() string {
	return "NewCountryLogin"
}

func (r *LoginLocationRule) Evaluate(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "SSHD_Accepted_Password" {
		return nil
	}

	user := event.Metadata["user"]
	country, ok := event.Metadata["country"]
	if !ok || user == "" {
		return nil // cant evaluate without user and country
	}

	s := state.LoginLocations
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.Locations[user]; !ok {
		s.Locations[user] = make(map[string]bool)
	}

	if !s.Locations[user][country] {
		s.Locations[user][country] = true
		return &model.Alert{
			RuleName:  r.Name(),
			Message:   fmt.Sprintf("User '%s' logged in from a new country: %s (IP: %s)", user, country, event.Source),
			Severity:  "MEDIUM",
			Timestamp: event.Timestamp,
			Source:    event.Source,
			Metadata: map[string]string{
				"user":    user,
				"country": country,
			},
		}
	}

	return nil
}

// --- Rapid Execution Rule ----
type RapidProcessExecutionRule struct {
	Threshold int
	Window    time.Duration
}

func NewRapidProcessExecutionRuile() Rule {
	return &RapidProcessExecutionRule{
		Threshold: 10,
		Window:    30 * time.Second,
	}
}

func (r *RapidProcessExecutionRule) Name() string {
	return "RapidProcessExecution"
}

func (r *RapidProcessExecutionRule) Evaluate(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	s := state.ProcessExecutionHistory
	s.mu.Lock()
	defer s.mu.Unlock()

	source := event.Source
	now := event.Timestamp
	cutoff := now.Add(-r.Window)

	currentProc := ProcessExecution{
		Timestamp:   event.Timestamp,
		ProcessName: event.Metadata["process_name"],
		Command:     event.Metadata["command"],
		PID:         event.Metadata["pid"],
		PPID:        event.Metadata["ppid"],
		UID:         event.Metadata["uid"],
	}

	history := append(s.History[source], currentProc)

	var recentHistory []ProcessExecution
	for _, proc := range history {
		if proc.Timestamp.After(cutoff) {
			recentHistory = append(recentHistory, proc)
		}
	}

	s.History[source] = recentHistory
	recentCount := len(recentHistory)

	if recentCount >= r.Threshold {
		return &model.Alert{
			RuleName:  r.Name(),
			Message:   fmt.Sprintf("Detected %d processes executed in %s from %s", recentCount, r.Window, source),
			Severity:  "MEDIUM",
			Timestamp: event.Timestamp,
			Source:    source,
			Metadata: map[string]string{
				"process_count": fmt.Sprintf("%d", recentCount),
				"time_window":   r.Window.String(),
			},
		}
	}

	return nil
}

// -- IP Watchlist Rules ---

type IPWatchlistRule struct{}

func NewIPWatchlistRule() Rule {
	return &IPWatchlistRule{}
}

func (r *IPWatchlistRule) Name() string {
	return "IPWatchlistMatch"
}

func (r *IPWatchlistRule) Evaluate(event model.Event, state *StateManager) *model.Alert {
	if event.Source == "" {
		return nil // cant check events without a source IP
	}

	s := state.IPWatchlist
	s.mu.Lock()
	isMatch, found := s.Watchlist[event.Source]
	s.mu.Unlock()

	if found && isMatch {
		return &model.Alert{
			RuleName:  r.Name(),
			Message:   fmt.Sprintf("Traffic detected from a known bad IP address: %s", event.Source),
			Severity:  "HIGH",
			Timestamp: event.Timestamp,
			Source:    event.Source,
			Metadata: map[string]string{
				"mitre_tactic": "TA0011",
				"event_type":   event.EventType,
			},
		}
	}

	return nil
}
