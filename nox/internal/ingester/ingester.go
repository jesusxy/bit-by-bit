package ingester

import (
	"log"
	"log/slog"
	"nox/internal/model"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

// Example Logs:
// Aug 24 13:30:00 my-server sshd[8888]: Accepted password for jsmith from 192.168.1.50 port 12345 ssh2
// type=EXECVE msg=audit(1692889800.123:1): argc=3 a0="ls" a1="-la" a2="/tmp" pid=1234 ppid=567 auid=0 uid=0 gid=0

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
		Regex:     regexp.MustCompile(`Accepted password for (\S+) from ([\d\.]+) port \d+ ssh2`),
		SimpleBuilder: func(matches []string) (model.Event, error) {
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
		Regex:     regexp.MustCompile(`type=EXECVE msg=audit\(([0-9.]+):\d+\):(.*?)pid=(\d+) ppid=(\d+).*?uid=(\d+) gid=(\d+)`),
		AuditBuilder: func(matches []string, auditLine string) (model.Event, error) {
			timestampFloat, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				timestampFloat = float64(time.Now().Unix())
			}
			timestamp := time.Unix(int64(timestampFloat), int64((timestampFloat-float64(int64(timestampFloat)))*1e9))

			command, args := extractCommandFromAudit(auditLine)
			fullCommand := command

			if len(args) > 0 {
				fullCommand = command + " " + strings.Join(args, " ")
			}

			return model.Event{
				Timestamp: timestamp,
				EventType: "Process_Executed",
				Source:    "localhost",
				Metadata: map[string]string{
					"process_name": command,
					"pid":          matches[3],
					"ppid":         matches[4],
					"uid":          matches[5],
					"gid":          matches[6],
					"command":      fullCommand,
					"args":         strings.Join(args, " "),
				},
			}, nil
		},
	},
}

func extractCommandFromAudit(auditLine string) (command string, args []string) {
	argcRegex := regexp.MustCompile(`argc=(\d+)`)
	argcMatches := argcRegex.FindStringSubmatch(auditLine)

	if len(argcMatches) < 2 {
		return "", nil
	}

	argc, err := strconv.Atoi(string(argcMatches[1]))
	if err != nil {
		return "", nil
	}

	var allArgs []string
	for i := 0; i < argc; i++ {
		argRegex := regexp.MustCompile(`a` + strconv.Itoa(i) + `="([^"]*)`)
		argMatches := argRegex.FindStringSubmatch(auditLine)
		if len(argMatches) >= 2 {
			allArgs = append(allArgs, argMatches[1])
		}
	}

	if len(allArgs) == 0 {
		return "", nil
	}

	command = allArgs[0]
	if len(allArgs) > 1 {
		args = allArgs[1:]
	}

	return command, args
}

func ParseLog(logln string) (model.Event, error) {
	for _, parser := range parsers {
		matches := parser.Regex.FindStringSubmatch(logln)

		if len(matches) > 0 {
			if parser.SimpleBuilder != nil {
				return parser.SimpleBuilder(matches)
			} else if parser.AuditBuilder != nil {
				return parser.AuditBuilder(matches, logln)
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
