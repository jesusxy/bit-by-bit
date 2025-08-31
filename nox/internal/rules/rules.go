package rules

import (
	"fmt"
	"log/slog"
	"nox/internal/model"
	"os"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"
)

type Rule func(event model.Event, state *StateManager) *model.Alert

type StateManager struct {
	mu sync.RWMutex
	// Tracks failed login attempts. Key: IP Address, Value: List of timestamps.
	FailedLoginAttempts map[string][]time.Time
	// we can add more state maps here for future rules, e.g.:
	UserLoginLocations map[string]map[string]bool // Key: Username, Value: Set of country codes
	// Track process execution patterns for behavioral analysis
	ProcessExecutionHistory map[string][]ProcessExecution
	// Track suspicious command frequency per source
	SuspiciousCommandCount map[string]int
	BruteForceAlertedIPs   map[string]bool
}

type ProcessExecution struct {
	Timestamp   time.Time
	ProcessName string
	Command     string
	PID         string
	PPID        string
	UID         string
}

type Condition struct {
	Field    string `yaml:"field"`
	Operator string `yaml:"opearator"`
	Value    string `yaml:"value"`
}

type RuleDefinition struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	TechniqueID string `yaml:"technique_id"`
	Severity    string `yaml:"severity"`
	EventType   string `yaml:"event_type"`
	Conditions  []Condition
}

func NewStateManager() *StateManager {
	return &StateManager{
		FailedLoginAttempts:     make(map[string][]time.Time),
		UserLoginLocations:      make(map[string]map[string]bool),
		ProcessExecutionHistory: make(map[string][]ProcessExecution),
		SuspiciousCommandCount:  make(map[string]int),
		BruteForceAlertedIPs:    make(map[string]bool),
	}
}

func LoadRulesFromFile(path string) ([]RuleDefinition, error) {
	slog.Info("Loading detection rules from file", "path", path)

	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var rules []RuleDefinition
	err = yaml.Unmarshal(file, &rules)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal rules yaml: %w", err)
	}

	slog.Info("Successfully loaded detection rules", "count", len(rules))
	return rules, nil
}

func checkConditions(event model.Event, rule RuleDefinition) bool {
	for _, cond := range rule.Conditions {
		fieldParts := strings.Split(cond.Field, ".")
		if len(fieldParts) != 2 || fieldParts[0] != "metadata" {
			continue
		}

		eventValue, ok := event.Metadata[fieldParts[1]]
		if !ok {
			return false
		}

		match := false

		switch cond.Operator {
		case "contains":
			match = strings.Contains(eventValue, cond.Value)
		case "equals":
			match = (eventValue == cond.Value)
		default:
			match = false
		}

		if !match {
			return false
		}
	}

	return true
}

func checkFailedLogins(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "SSHD_Failed_Password" {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()

	//--- state logic ----//
	ip := event.Source
	const (
		threshold = 5
		window    = 60 * time.Second
	)

	// prune old timestamps that are outside the time window
	var recentAttempts []time.Time
	now := time.Now().UTC()
	for _, t := range state.FailedLoginAttempts[ip] {
		if now.Sub(t) <= window {
			recentAttempts = append(recentAttempts, t)
		}
	}

	// add new attempt
	recentAttempts = append(recentAttempts, event.Timestamp)
	state.FailedLoginAttempts[ip] = recentAttempts

	if state.BruteForceAlertedIPs[ip] {
		return nil
	}

	if len(recentAttempts) >= threshold {
		state.BruteForceAlertedIPs[ip] = true

		return &model.Alert{
			RuleName:  "TooManyFailedLogins",
			Message:   fmt.Sprintf("Detected %d failed SSH logins from %s in the last minute.", len(recentAttempts), ip),
			Severity:  "HIGH",
			Timestamp: event.Timestamp,
			Source:    ip,
			Metadata: map[string]string{
				"attempt_count": fmt.Sprintf("%d", len(recentAttempts)),
				"time_window":   window.String(),
			},
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
			RuleName:  "NewCountryLogin",
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

var suspiciousCommands = map[string]string{
	"netcat":     "network_tool",
	"usermod":    "user_management",
	"userdel":    "user_management",
	"bash -i":    "reverse_shell",
	"sh -i":      "reverse_shell",
	"perl -e":    "script_execution",
	"history -c": "anti_forensics",
	"dd if=":     "disk_access",
	"/dev/tcp/":  "network_connection",
}

func checkSuspiciousCommands(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	command, ok := event.Metadata["command"]
	if !ok {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	processExec := ProcessExecution{
		Timestamp:   event.Timestamp,
		ProcessName: event.Metadata["process_name"],
		Command:     command,
		PID:         event.Metadata["pid"],
		PPID:        event.Metadata["ppid"],
		UID:         event.Metadata["uid"],
	}

	source := event.Source
	state.ProcessExecutionHistory[source] = append(state.ProcessExecutionHistory[source], processExec)

	for suspiciousCmd, category := range suspiciousCommands {
		if strings.Contains(strings.ToLower(command), strings.ToLower(suspiciousCmd)) {
			state.SuspiciousCommandCount[source]++

			severity := "MEDIUM"
			if category == "reverse_shell" || category == "permission_escalation" {
				severity = "HIGH"
			}

			return &model.Alert{
				RuleName:  "SuspiciousCommandExecution",
				Message:   fmt.Sprintf("A suspicious command was executed: '%s'", command),
				Severity:  severity,
				Timestamp: event.Timestamp,
				Source:    source,
				Metadata: map[string]string{
					"command":      command,
					"category":     category,
					"process_name": event.Metadata["process_name"],
					"pid":          event.Metadata["pid"],
					"uid":          event.Metadata["uid"],
				},
			}
		}
	}
	return nil
}

func checkRapidProcessExecution(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	source := event.Source
	const (
		threshold = 10
		window    = 30 * time.Second
	)

	history := state.ProcessExecutionHistory[source]
	if len(history) == 0 {
		return nil
	}

	cutoff := event.Timestamp.Add(-window)
	recentCount := 0
	for _, proc := range history {
		if proc.Timestamp.After(cutoff) {
			recentCount++
		}
	}

	if recentCount >= threshold {
		return &model.Alert{
			RuleName:  "RapidProcessExecution",
			Message:   fmt.Sprintf("Detected %d processes executed in %s from %s", recentCount, window, source),
			Severity:  "MEDIUM",
			Timestamp: event.Timestamp,
			Source:    source,
			Metadata: map[string]string{
				"process_count": fmt.Sprintf("%d", recentCount),
				"time_window":   window.String(),
			},
		}
	}

	return nil
}

func checkPrivilegedEscalation(event model.Event, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	command := event.Metadata["command"]
	uid := event.Metadata["uid"]
	ppid := event.Metadata["ppid"]

	privEscPatterns := []string{
		"sudo su",
		"su -",
		"sudo -i",
		"sudo bash",
		"sudo sh",
	}

	for _, pattern := range privEscPatterns {
		if strings.Contains(strings.ToLower(command), pattern) {
			return &model.Alert{
				RuleName:  "PrivilegeEscalationAttempt",
				Message:   fmt.Sprintf("Potential privilege escalation detected: '%s' (UID: %s, PPID: %s)", command, uid, ppid),
				Severity:  "HIGH",
				Timestamp: event.Timestamp,
				Source:    event.Source,
				Metadata: map[string]string{
					"command": command,
					"uid":     uid,
					"ppid":    ppid,
				},
			}
		}
	}

	return nil
}

// activeRules is the registry of all rules the engine will run.
var activeRules = []Rule{
	checkFailedLogins,
	checkNewCountryLogins,
	checkRapidProcessExecution,
}

func EvaluateEvent(event model.Event, yamlRules []RuleDefinition, state *StateManager) []model.Alert {
	var triggeredAlerts []model.Alert

	for _, rule := range yamlRules {
		if event.EventType != rule.EventType {
			continue
		}

		if checkConditions(event, rule) {
			alert := model.Alert{
				RuleName:  rule.Name,
				Message:   rule.Description,
				Severity:  rule.Severity,
				Timestamp: event.Timestamp,
				Source:    event.Source,
				Metadata: map[string]string{
					"mitre_technique_id": rule.TechniqueID,
					"command":            event.Metadata["command"],
					"process_name":       event.Metadata["process_name"],
					"pid":                event.Metadata["pid"],
				},
			}
			triggeredAlerts = append(triggeredAlerts, alert)
		}
	}

	for _, rule := range activeRules {
		if alert := rule(event, state); alert != nil {
			triggeredAlerts = append(triggeredAlerts, *alert)
		}
	}

	return triggeredAlerts
}
