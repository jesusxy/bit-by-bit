package ingester

import (
	"context"
	"log"
	"log/slog"
	"nox/internal/model"

	"github.com/hpcloud/tail"
)

type Parser interface {
	Parse(logLine string) (model.Event, error)
}

type Ingester struct {
	logger  *slog.Logger
	parsers []Parser
}

func NewIngester(logger *slog.Logger) *Ingester {
	return &Ingester{
		logger: logger,
		parsers: []Parser{
			NewSSHDParser(),
			NewExecsnoopParser(),
		},
	}
}

func (i *Ingester) ParseLog(logline string) (model.Event, error) {
	for _, parser := range i.parsers {
		event, err := parser.Parse(logline)
		if err == model.ErrIgnoredLine {
			continue
		}
		return event, err
	}

	return model.Event{}, model.ErrIgnoredLine
}

func (i *Ingester) TailFile(ctx context.Context, fpath string, ch chan<- model.Event) {
	t, err := tail.TailFile(fpath, tail.Config{Follow: true, ReOpen: true, Logger: tail.DiscardingLogger})
	if err != nil {
		log.Fatalf("failed to tail file: %v", err)
	}

	i.logger.Info("Started tailling log file", "path", fpath)

	for {
		select {
		case <-ctx.Done():
			t.Stop()
			i.logger.Info("Stopping log file tailing due to context cancellation.", "path", fpath)
			return
		case line := <-t.Lines:
			if line == nil {
				continue
			}
			if line.Text == "" {
				continue
			}

			event, err := i.ParseLog(line.Text)
			if err == model.ErrIgnoredLine {
				i.logger.Debug("Ignoring log line", "line", line.Text)
				continue
			} else if err != nil {
				slog.Error("failed to parse line", "error", err, "line", line.Text)
				continue
			}
			slog.Debug("Parsed event", "type", event.EventType, "source", event.Source)
			ch <- event
		}
	}
}
