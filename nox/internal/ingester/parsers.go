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

type sshdParser struct {
	failedLoginRegex   *regexp.Regexp
	acceptedLoginRegex *regexp.Regexp
}

func NewSSHDParser() Parser {
	return &sshdParser{
		failedLoginRegex:   regexp.MustCompile(`Failed password for .*?(\S+) from ([\d\.]+) port \d+ ssh2`),
		acceptedLoginRegex: regexp.MustCompile(`sshd\[(\d+)\]: Accepted password for (\S+) from ([\d\.]+) port \d+ ssh2`),
	}
}

func parseLogTimestamp(logLine string) (time.Time, error) {
	timestampStr := strings.Join(strings.Fields(logLine)[0:3], " ")
	t, err := time.Parse("Jan 2 15:04:05", timestampStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("could not parse timestamp: %w", err)
	}

	return t.AddDate(time.Now().UTC().Year(), 0, 0), nil
}

func (p *sshdParser) Parse(logLine string) (model.Event, error) {
	if matches := p.failedLoginRegex.FindStringSubmatch(logLine); len(matches) > 0 {
		ts, err := parseLogTimestamp(logLine)
		if err != nil {
			ts = time.Now().UTC()
		}

		return model.Event{
			Timestamp: ts,
			EventType: "SSHD_Failed_Password",
			Source:    matches[2],
			Metadata:  map[string]string{"user": matches[1]},
		}, nil
	}

	if matches := p.acceptedLoginRegex.FindStringSubmatch(logLine); len(matches) > 0 {
		ts, err := parseLogTimestamp(logLine)
		if err != nil {
			ts = time.Now().UTC()
		}

		return model.Event{
			Timestamp: ts,
			EventType: "SSHD_Accepted_Password",
			Metadata: map[string]string{
				"user":     matches[2],
				"sshd_pid": matches[1],
			},
		}, nil
	}

	return model.Event{}, model.ErrIgnoredLine
}

type execsnoopParser struct {
	regex *regexp.Regexp
}

func NewExecsnoopParser() Parser {
	regex := regexp.MustCompile(`^\S+\s+(\d+)\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.*)$`)
	return &execsnoopParser{
		regex: regex,
	}
}

func (p *execsnoopParser) Parse(logLine string) (model.Event, error) {
	matches := p.regex.FindStringSubmatch(logLine)
	if len(matches) == 0 {
		return model.Event{}, model.ErrIgnoredLine
	}

	// matches[1]: UID
	// matches[2]: process name
	// matches[3]: PID
	// matches[4]: PPID
	// matches[5]: return code
	// matches[6]: full command
	return model.Event{
		Timestamp: time.Now().UTC(),
		EventType: "Process_Executed",
		Source:    "localhost",
		Metadata: map[string]string{
			"uid":          strings.TrimSpace(matches[1]),
			"process_name": strings.TrimSpace(matches[2]),
			"pid":          strings.TrimSpace(matches[3]),
			"ppid":         strings.TrimSpace(matches[4]),
			"return_code":  strings.TrimSpace(matches[5]),
			"command":      strings.TrimSpace(matches[6]),
		},
	}, nil
}
