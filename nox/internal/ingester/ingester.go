package ingester

import (
	"bufio"
	"fmt"
	"log"
	"nox/internal/model"
	"os"
)

func ParseLog(logln string) (model.Event, error) {

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
