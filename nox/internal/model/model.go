package model

import (
	"errors"
	"time"
)

var ErrIgnoredLine = errors.New("log line does not match any known patterns")

type Alert struct {
	RuleName  string
	Message   string
	Severity  string
	Timestamp time.Time
	Source    string
	Metadata  map[string]string
}

type Event struct {
	Timestamp time.Time
	EventType string
	Source    string
	Metadata  map[string]string
}

func (a *Alert) GetSeverityLevel() int {
	switch a.Severity {
	case "LOW":
		return 1
	case "MEDIUM":
		return 2
	case "HIGH":
		return 3
	case "CRITICAL":
		return 4
	default:
		return 0
	}
}

func (a *Alert) IsHighPriority() bool {
	return a.GetSeverityLevel() >= 3
}
