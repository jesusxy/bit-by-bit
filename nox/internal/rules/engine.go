package rules

import (
	"log/slog"
	"nox/internal/model"
)

type Rule interface {
	Name() string
	Evaluate(event model.Event, state *StateManager) *model.Alert
}

// A CorrelationRule looks for chains of events and alerts over time.
type CorrelationRule interface {
	Name() string
	Evaluate(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert
}

type Engine struct {
	logger           *slog.Logger
	state            *StateManager
	statelessRules   []RuleDefinition
	statefulRules    []Rule
	correlationRules []CorrelationRule
}

func NewEngine(logger *slog.Logger, state *StateManager, yamlRules []RuleDefinition) *Engine {
	return &Engine{
		logger:         logger,
		state:          state,
		statelessRules: yamlRules,
		statefulRules: []Rule{
			NewFailedLoginsRule(),
			NewLoginLocationRule(),
			NewRapidProcessExecutionRuile(),
			NewIPWatchlistRule(),
		},
		correlationRules: []CorrelationRule{
			NewBruteForceAndEvasionRule(),
			NewDownloadAndExecuteRule(),
			NewLocalAccountImmediateUseRule(),
			NewLoginAndEscalationRule(),
		},
	}
}

func (e *Engine) EvaluateEvent(event model.Event) []model.Alert {
	var triggeredAlerts []model.Alert

	for _, rule := range e.statelessRules {
		if event.EventType == rule.EventType && EvaluateYAMLRule(event, rule) {
			alert := model.Alert{
				RuleName:  rule.Name,
				Message:   rule.Description,
				Severity:  rule.Severity,
				Timestamp: event.Timestamp,
				Source:    event.Source,
				Metadata: map[string]string{
					"mitre_technique_id": rule.TechniqueID,
					"pid":                event.Metadata["pid"],
				},
			}

			if cmd, ok := event.Metadata["command"]; ok {
				alert.Metadata["command"] = cmd
			}

			if processName, ok := event.Metadata["process_name"]; ok {
				alert.Metadata["process_name"] = processName
			}

			triggeredAlerts = append(triggeredAlerts, alert)
		}
	}

	for _, rule := range e.statefulRules {
		if alert := rule.Evaluate(event, e.state); alert != nil {
			triggeredAlerts = append(triggeredAlerts, *alert)
		}
	}

	for _, rule := range e.correlationRules {
		if alert := rule.Evaluate(event, triggeredAlerts, e.state); alert != nil {
			triggeredAlerts = append(triggeredAlerts, *alert)
		}
	}

	return triggeredAlerts
}
