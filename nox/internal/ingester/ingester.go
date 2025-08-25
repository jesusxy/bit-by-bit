package ingester

import (
	"log"
	"log/slog"
	"nox/internal/model"
	"regexp"
	"time"

	"github.com/hpcloud/tail"
)

// Example Logs:
// Aug 24 13:30:00 my-server sshd[8888]: Accepted password for jsmith from 192.168.1.50 port 12345 ssh2
// type=EXECVE msg=audit(1692889800.123:1): argc=3 a0="ls" a1="-la" a2="/tmp" pid=1234 ppid=567 auid=0 uid=0 gid=0

type logParser struct {
	EventType string
	Regex     *regexp.Regexp
	Builder   func(matches []string) (model.Event, error)
}

var parsers = []logParser{
	{
		EventType: "SSHD_Failed_Password",
		Regex:     regexp.MustCompile(`Failed password for .*?(\S+) from ([\d\.]+) port \d+ ssh2`),
		Builder: func(matches []string) (model.Event, error) {
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
		Regex:     regexp.MustCompile(`Accepted password for (\S+) from ([\d\.]+) port \d+ ssh2`),
		Builder: func(matches []string) (model.Event, error) {
			return model.Event{
				Timestamp: time.Now().UTC(),
				EventType: "SSHD_Accepted_Password",
				Source:    matches[2],
				Metadata:  map[string]string{"user": matches[1]},
			}, nil
		},
	},
	{
		EventType: "Process_Executed",
		Regex:     regexp.MustCompile(`\d{2}:\d{2}:\d{2}\s+(\S+)\s+(\d+)\s+.*\s+0\s+(.*)`),
		Builder: func(matches []string) (model.Event, error) {
			return model.Event{
				Timestamp: time.Now().UTC(),
				EventType: "Process_Executed",
				Source:    "localhost",
				Metadata: map[string]string{
					"process_name": matches[1],
					"pid":          matches[2],
					"command":      matches[3],
				},
			}, nil
		},
	},
}

func ParseLog(logln string) (model.Event, error) {
	for _, parser := range parsers {
		matches := parser.Regex.FindStringSubmatch(logln)

		if len(matches) > 0 {
			return parser.Builder(matches)
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
