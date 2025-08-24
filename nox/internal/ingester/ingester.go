package ingester

import (
	"bufio"
	"fmt"
	"log"
	"log/slog"
	"nox/internal/model"
	"os"
	"regexp"
	"time"

	"github.com/hpcloud/tail"
)

// Example Logs:
// Aug 24 13:30:00 my-server sshd[8888]: Accepted password for jsmith from 192.168.1.50 port 12345 ssh2

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

func ReadFile(fpath string) ([]model.Event, error) {
	// open the file
	f, err := os.Open(fpath)
	if err != nil {
		return nil, fmt.Errorf("error opening file from: %s:%w", fpath, err)
	}
	defer f.Close()

	var events []model.Event

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		logLine := scanner.Text()

		event, err := ParseLog(logLine)
		if err == model.ErrIgnoredLine {
			continue
		} else if err != nil {
			slog.Error("Failed to parse log line", "error", err)
			continue
		}

		events = append(events, event)
	}

	return events, nil
}

func TailFile(fpath string, ch chan<- model.Event) {
	t, err := tail.TailFile(fpath, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		log.Fatalf("failed to tail file: %v", err)
	}

	for line := range t.Lines {
		event, err := ParseLog(line.Text)

		if err == model.ErrIgnoredLine {
			continue
		} else if err != nil {
			slog.Error("failed to parse line", "error", err)
			continue
		}

		ch <- event
	}
}
