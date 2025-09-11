package rules

import (
	"bufio"
	"fmt"
	"log/slog"
	"nox/internal/model"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Rule func(event model.Event, state *StateManager) *model.Alert

// A CorrelationRule looks for chains of events and alerts over time.
type CorrelationRule func(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert

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

type PostBruteForceInfo struct {
	LoginTime time.Time
	SourceIP  string
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
