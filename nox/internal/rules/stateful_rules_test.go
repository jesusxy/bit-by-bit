package rules

import (
	"nox/internal/model"
	"testing"
	"time"
)

func TestFailedLoginsRuleEvaluate(t *testing.T) {
	baseTime := time.Date(2026, time.June, 19, 12, 0, 0, 0, time.UTC)
	sourceIP := "203.0.113.10"

	tests := []struct {
		name             string
		attemptOffsets   []time.Duration
		wantAlertCount   int
		wantRuleName     string
		wantSeverity     string
		wantAttemptCount string
	}{
		{
			name:           "fewer than threshold within window does not alert",
			attemptOffsets: []time.Duration{0, 10 * time.Second, 20 * time.Second, 30 * time.Second},
			wantAlertCount: 0,
		},
		{
			name:             "threshold within window alerts",
			attemptOffsets:   []time.Duration{0, 10 * time.Second, 20 * time.Second, 30 * time.Second, 40 * time.Second},
			wantAlertCount:   1,
			wantRuleName:     "TooManyFailedLogins",
			wantSeverity:     "HIGH",
			wantAttemptCount: "5",
		},
		{
			name:           "attempts outside window do not alert",
			attemptOffsets: []time.Duration{0, 61 * time.Second, 122 * time.Second, 183 * time.Second, 244 * time.Second},
			wantAlertCount: 0,
		},
		{
			name:             "duplicate alerts are suppressed after first alert",
			attemptOffsets:   []time.Duration{0, 10 * time.Second, 20 * time.Second, 30 * time.Second, 40 * time.Second, 50 * time.Second},
			wantAlertCount:   1,
			wantRuleName:     "TooManyFailedLogins",
			wantSeverity:     "HIGH",
			wantAttemptCount: "5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewFailedLoginsRule()
			state := NewStateManager()

			var alerts []model.Alert
			for _, offset := range tt.attemptOffsets {
				alert := rule.Evaluate(failedLoginEvent(baseTime.Add(offset), sourceIP, "root"), state)
				if alert != nil {
					alerts = append(alerts, *alert)
				}
			}

			if len(alerts) != tt.wantAlertCount {
				t.Fatalf("got %d alerts, want %d", len(alerts), tt.wantAlertCount)
			}

			if tt.wantAlertCount == 0 {
				return
			}

			alert := alerts[len(alerts)-1]
			if alert.RuleName != tt.wantRuleName {
				t.Fatalf("got rule name %q, want %q", alert.RuleName, tt.wantRuleName)
			}
			if alert.Severity != tt.wantSeverity {
				t.Fatalf("got severity %q, want %q", alert.Severity, tt.wantSeverity)
			}
			if alert.Source != sourceIP {
				t.Fatalf("got source %q, want %q", alert.Source, sourceIP)
			}
			if alert.Metadata["attempt_count"] != tt.wantAttemptCount {
				t.Fatalf("got attempt_count %q, want %q", alert.Metadata["attempt_count"], tt.wantAttemptCount)
			}
		})
	}
}

func TestFailedLoginsRuleEvaluate_DifferentUsersSameIPDoesNotAlert(t *testing.T) {
	rule := NewFailedLoginsRule()
	state := NewStateManager()
	baseTime := time.Date(2026, time.June, 19, 12, 0, 0, 0, time.UTC)
	sourceIP := "203.0.113.10"

	users := []string{"root", "admin", "deploy", "postgres", "ubuntu"}

	for i, user := range users {
		alert := rule.Evaluate(failedLoginEvent(baseTime.Add(time.Duration(i)*time.Second), sourceIP, user), state)
		if alert != nil {
			t.Fatalf("got alert for mixed users, want none")
		}
	}
}

func failedLoginEvent(timestamp time.Time, source, user string) model.Event {
	return model.Event{
		Timestamp: timestamp,
		EventType: "SSHD_Failed_Password",
		Source:    source,
		Metadata: map[string]string{
			"user": user,
		},
	}
}
