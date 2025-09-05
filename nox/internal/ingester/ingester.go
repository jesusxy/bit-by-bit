package ingester

import (
	"log"
	"log/slog"
	"nox/internal/model"
	"regexp"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

// Example Logs:
// Aug 24 13:30:00 my-server sshd[8888]: Accepted password for jsmith from 192.168.1.50 port 12345 ssh2
// type=EXECVE msg=audit(1692889800.123:1): argc=3 a0="ls" a1="-la" a2="/tmp" pid=1234 ppid=567 auid=0 uid=0 gid=0
/** execsnoop output format
TIME(s)  PID    PPID   RET ARGS
0.000    1234   567    0   /bin/ls -la /tmp
1.245    1235   567    0   /usr/bin/whoami
2.891    1236   567    0   /bin/ps aux
*/

type SimpleBuilder func(matches []string) (model.Event, error)
type AuditBuilder func(matches []string, auditLine string) (model.Event, error)

type logParser struct {
	EventType     string
	Regex         *regexp.Regexp
	SimpleBuilder SimpleBuilder
	AuditBuilder  AuditBuilder
}

var parsers = []logParser{
	{
		EventType: "SSHD_Failed_Password",
		Regex:     regexp.MustCompile(`Failed password for .*?(\S+) from ([\d\.]+) port \d+ ssh2`),
		SimpleBuilder: func(matches []string) (model.Event, error) {
			return model.Event{
				Timestamp: time.Now().UTC(),
				EventType: "SSHD_Failed_Password",
				Source:    matches[2],
				Metadata:  map[string]string{"user": matches[1]},
			}, nil
		},
	},
	{
		EventType: "SSHD_Accepted_Password",
		Regex:     regexp.MustCompile(`sshd\[(\d+)\]: Accepted password for (\S+) from ([\d\.]+) port \d+ ssh2`),
		SimpleBuilder: func(matches []string) (model.Event, error) {
			return model.Event{
				Timestamp: time.Now().UTC(),
				EventType: "SSHD_Accepted_Password",
				Source:    matches[3],
				Metadata: map[string]string{
					"user":     matches[2],
					"sshd_pid": matches[1],
				},
			}, nil
		},
	},
	{
		EventType: "Process_Executed",
		Regex:     regexp.MustCompile(`^\S+\s+(\d+)\s+(\S+)\s+(\d+)\s+(\d+)\s+(\d+)\s+(.*)$`),
		SimpleBuilder: func(matches []string) (model.Event, error) {
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
		},
	},
}

func ParseLog(logline string) (model.Event, error) {
	for _, parser := range parsers {
		matches := parser.Regex.FindStringSubmatch(logline)

		if len(matches) > 0 {
			if parser.SimpleBuilder != nil {
				return parser.SimpleBuilder(matches)
			} else if parser.AuditBuilder != nil {
				return parser.AuditBuilder(matches, logline)
			}
		}
	}

	return model.Event{}, model.ErrIgnoredLine
}

func TailFile(fpath string, ch chan<- model.Event) {
	t, err := tail.TailFile(fpath, tail.Config{Follow: true, ReOpen: true, Logger: tail.DiscardingLogger})
	if err != nil {
		log.Fatalf("failed to tail file: %v", err)
	}

	slog.Info("Started tailling log file", "path", fpath)

	for line := range t.Lines {
		if line.Text == "" {
			continue // skip empty lines
		}

		event, err := ParseLog(line.Text)

		if err == model.ErrIgnoredLine {
			slog.Debug("Ignoring log line", "line", line.Text)
			continue
		} else if err != nil {
			slog.Error("failed to parse line", "error", err, "line", line.Text)
			continue
		}

		slog.Debug("Parsed event", "type", event.EventType, "source", event.Source)
		ch <- event
	}

	slog.Warn("Log file tailing stopped", "path", fpath)
}
