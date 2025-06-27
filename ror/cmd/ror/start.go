package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli/v3"
)

func newStartCmd() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "Start a created container",
		ArgsUsage: "<id>",
		Action: func(c context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() < 1 {
				return cli.Exit("container id required", 1)
			}

			fmt.Printf("Running container %s\n", cmd.Args().First())
			id := cmd.Args().First()

			return startContainer(id)
		},
	}
}

func startContainer(id string) error {
	// locate the container state via id
	containerState := filepath.Join(defaultBasePath, id)

	if _, err := os.Stat(containerState); os.IsNotExist(err) {
		// If we get an "IsNotExist" error, it means the directory isn't there.
		return fmt.Errorf("container with id '%s' does not exist", id)
	}

	// Load the blueprint (config.json)
	configJSON, err := os.ReadFile(filepath.Join(containerState, "config.json"))
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	// unmarshall config into spec struct
	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshall bundle into OCI spec: %w", err)
	}

	log.Printf("Successfully loaded spec for container '%s'. Starting...", id)

	return nil
}
