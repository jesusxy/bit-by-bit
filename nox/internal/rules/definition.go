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

type ProcessExecution struct {
	Timestamp   time.Time
	ProcessName string
	Command     string
	PID         string
	PPID        string
	UID         string
}

type PostBruteForceInfo struct {
	LoginTime time.Time
	SourceIP  string
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

func LoadIPWatchlistFromFile(path string) (map[string]bool, error) {
	slog.Info("Loading IP watchlist from file...", "path", path)

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read IP watchlist file: %w", err)
	}
	defer file.Close()

	ipWatchlist := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		ip := strings.TrimSpace(scanner.Text())
		if ip != "" && !strings.HasPrefix(ip, "#") {
			ipWatchlist[ip] = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	slog.Info("Successfully loaded IP watchlist", "count", len(ipWatchlist))
	return ipWatchlist, nil
}

func EvaluateYAMLRule(event model.Event, rule RuleDefinition) bool {
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
