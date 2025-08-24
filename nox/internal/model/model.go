package model

import (
	"errors"
	"time"
)

var ErrIgnoredLine = errors.New("log line does not match any known patterns")

type Alert struct {
	RuleName string
	Message  string
}

type Event struct {
	Timestamp time.Time
	EventType string
	Source    string
	Metadata  map[string]string
}
