package ingester

import (
	"fmt"
	"nox/internal/model"
	"regexp"
	"strings"
	"time"
)

// Example Logs:
// Aug 24 13:30:00 my-server sshd[8888]: Accepted password for jsmith from 192.168.1.50 port 12345 ssh2
// type=EXECVE msg=audit(1692889800.123:1): argc=3 a0="ls" a1="-la" a2="/tmp" pid=1234 ppid=567 auid=0 uid=0 gid=0
/** execsnoop output format
*	TIME(s)  PID    PPID   RET ARGS
*	0.000    1234   567    0   /bin/ls -la /tmp
*	1.245    1235   567    0   /usr/bin/whoami
*	2.891    1236   567    0   /bin/ps aux
**/

const (
	sshdTimeFormat      = "Jan _2 15:04:05"
	execsnoopTimeFormat = time.RFC3339
)

type sshdParser struct {
	failedLoginRegex   *regexp.Regexp
	acceptedLoginRegex *regexp.Regexp
}

func NewSSHDParser() Parser {
	return &sshdParser{
		failedLoginRegex:   regexp.MustCompile(`^(\w+\s+\d+\s+[\d:]+)\s+my-server sshd\[\d+\]: Failed password for .*?(\S+) from ([\d\.]+)`),
		acceptedLoginRegex: regexp.MustCompile(`^(\w+\s+\d+\s+[\d:]+)\s+my-server sshd\[(\d+)\]: Accepted password for (\S+) from ([\d\.]+)`),
	}
}

func (p *sshdParser) Parse(logLine string) (model.Event, error) {
	if matches := p.failedLoginRegex.FindStringSubmatch(logLine); len(matches) > 0 {
		ts, err := p.parseSSHDTimestamp(matches[1])
		if err != nil {
			return model.Event{}, fmt.Errorf("failed to parse sshd timestamp: %w", err)
		}

		return model.Event{
			Timestamp: ts,
			EventType: "SSHD_Failed_Password",
			Source:    matches[3],
			Metadata:  map[string]string{"user": matches[2]},
		}, nil
	}

	if matches := p.acceptedLoginRegex.FindStringSubmatch(logLine); len(matches) > 0 {
		ts, err := p.parseSSHDTimestamp(matches[1])
		if err != nil {
			return model.Event{}, fmt.Errorf("failed to parse sshd timestamp: %w", err)
		}

		return model.Event{
			Timestamp: ts,
			EventType: "SSHD_Accepted_Password",
			Source:    matches[4],
			Metadata: map[string]string{
				"user":     matches[3],
				"sshd_pid": matches[2],
			},
		}, nil
	}

	return model.Event{}, model.ErrIgnoredLine
}

func (p *sshdParser) parseSSHDTimestamp(timstamp string) (time.Time, error) {
	ts, err := time.ParseInLocation(sshdTimeFormat, timstamp, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse sshd timestamp: %w", err)
	}

	if ts.Year() == 0 {
		ts = ts.AddDate(time.Now().UTC().Year(), 0, 0)
	}

	return ts, nil
}

type execsnoopParser struct {
	// Regex captures: 1=Timestamp, 2=UID, 3=ProcessName, 4=PID, 5=PPID, 6=Retval, 7=Args
	regex *regexp.Regexp
}

func NewExecsnoopParser() Parser {
	regex := regexp.MustCompile(`^([\dTZ:.\-]+)\s+(\d+)\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.*)$`)
	return &execsnoopParser{
		regex: regex,
	}
}

func (p *execsnoopParser) Parse(logLine string) (model.Event, error) {
	matches := p.regex.FindStringSubmatch(logLine)
	if len(matches) < 8 {
		return model.Event{}, model.ErrIgnoredLine
	}

	ts, err := time.Parse(execsnoopTimeFormat, matches[1])
	if err != nil {
		return model.Event{}, fmt.Errorf("failed to parse execsnoop timestamp: %w", err)
	}

	// matches[1]: UID
	// matches[2]: process name
	// matches[3]: PID
	// matches[4]: PPID
	// matches[5]: return code
	// matches[6]: full command
	return model.Event{
		Timestamp: ts,
		EventType: "Process_Executed",
		Source:    "127.0.0.1",
		Metadata: map[string]string{
			"uid":          strings.TrimSpace(matches[2]),
			"process_name": strings.TrimSpace(matches[3]),
			"pid":          strings.TrimSpace(matches[4]),
			"ppid":         strings.TrimSpace(matches[5]),
			"return_code":  strings.TrimSpace(matches[6]),
			"command":      strings.TrimSpace(matches[7]),
		},
	}, nil
}
