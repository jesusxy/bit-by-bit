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

func newCreateCmd() *cli.Command {
	var cfg ContainerConfig

	return &cli.Command{
		Name:      "create",
		Usage:     "Create an OCI container",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "bundle",
				Aliases:     []string{"b"},
				Value:       ".",
				Usage:       "Path to the OCI bundle",
				Destination: &cfg.Bundle,
			},
			&cli.StringFlag{
				Name:        "pid-file",
				Usage:       "write child pid to this file (optional)",
				Destination: &cfg.PIDFile,
			},
		},
		Action: func(c context.Context, cmd *cli.Command) error {
			cfg.ID = cmd.Args().First()
			cfg.BasePath = "./run/ror"

			return createContainer(cfg)
		},
	}
}

func createContainer(cfg ContainerConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("container id required")
	}

	containerStatePath := filepath.Join(cfg.BasePath, cfg.ID)
	bundleConfigPath := filepath.Join(cfg.Bundle, "config.json")

	if err := os.MkdirAll(containerStatePath, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	configJSON, err := os.ReadFile(bundleConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	var spec specs.Spec

	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("bundle config.json is not a valid OCI spec: %w", err)
	}

	newConfigPath := filepath.Join(containerStatePath, "config.json")
	if err := os.WriteFile(newConfigPath, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write config to state directory: %w", err)
	}

	log.Printf("Creating container {id:%s, bundle:%s, pidFile: %s}\n", cfg.ID, cfg.Bundle, cfg.PIDFile)
	return nil
}
