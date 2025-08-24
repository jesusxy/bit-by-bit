package ingester

import (
	"bufio"
	"fmt"
	"log"
	"nox/internal/model"
	"os"
	"regexp"
	"time"
)

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
			log.Printf("could not parse line: %v", err)
			continue
		}

		events = append(events, event)
	}

	return events, nil
}
