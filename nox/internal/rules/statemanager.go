package rules

import (
	"sync"
	"time"
)

type FailedLoginState struct {
	mu         sync.Mutex
	Attempts   map[string][]time.Time // Key: IP Address, Value: List of recent attempt timestamps.
	AlertedIPs map[string]bool        // Key: IP Address, Value: True if an alert has been fired.
}

type IPWatchlistState struct {
	mu        sync.Mutex
	Watchlist map[string]bool // Key: IP Address
}

func (s *IPWatchlistState) Set(watchlist map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Watchlist = watchlist
}

type LoginLocationState struct {
	mu        sync.Mutex
	Locations map[string]map[string]bool // Key: Username, Value: Set of country codes.
}

type NewAccountState struct {
	mu            sync.Mutex
	CreationTimes map[string]time.Time // Key: Username, Value: Timestamp of creation.
}

type PostBruteForceLoginState struct {
	mu               sync.Mutex
	SuccessfulLogins map[string]PostBruteForceInfo // Key: SSHD PID of the successful login session.
}

type ProcessExecutionHistoryState struct {
	mu      sync.Mutex
	History map[string][]ProcessExecution // Key: Source Host (e.g., "localhost")
}

type StagedPayloadState struct {
	mu       sync.Mutex
	Payloads map[string]time.Time // Key: Filepath, Value: Timestamp of download.
}

type SuspiciousLoginState struct {
	mu     sync.Mutex
	Logins map[string]time.Time // Key: Source IP, Value: Timestamp of the suspicious login.
}

type StateManager struct {
	FailedLogins            *FailedLoginState
	IPWatchlist             *IPWatchlistState
	LoginLocations          *LoginLocationState
	NewAccountTracker       *NewAccountState
	ProcessExecutionHistory *ProcessExecutionHistoryState
	PostBruteForceLogins    *PostBruteForceLoginState
	StagedPayloads          *StagedPayloadState
	SuspiciousLoginTracker  *SuspiciousLoginState
}

func NewStateManager() *StateManager {
	return &StateManager{
		FailedLogins: &FailedLoginState{
			Attempts:   make(map[string][]time.Time),
			AlertedIPs: make(map[string]bool),
		},
		IPWatchlist: &IPWatchlistState{
			Watchlist: make(map[string]bool),
		},
		LoginLocations: &LoginLocationState{
			Locations: make(map[string]map[string]bool),
		},
		NewAccountTracker: &NewAccountState{
			CreationTimes: make(map[string]time.Time),
		},
		PostBruteForceLogins: &PostBruteForceLoginState{
			SuccessfulLogins: make(map[string]PostBruteForceInfo),
		},
		ProcessExecutionHistory: &ProcessExecutionHistoryState{
			History: make(map[string][]ProcessExecution),
		},
		StagedPayloads: &StagedPayloadState{
			Payloads: make(map[string]time.Time),
		},
		SuspiciousLoginTracker: &SuspiciousLoginState{
			Logins: make(map[string]time.Time),
		},
	}
}
