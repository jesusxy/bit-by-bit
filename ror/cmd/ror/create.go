package main

import (
	"context"
	"fmt"
	"log"

	"github.com/urfave/cli/v3"
)

type CreateCmdConfig struct {
	ID      string
	Bundle  string
	PIDFile string
}

func newCreateCmd() *cli.Command {
	var cfg CreateCmdConfig

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

			return createContainer(cfg)
		},
	}
}

func createContainer(cfg CreateCmdConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("container id required")
	}

	log.Printf("Creating container {id:%s, bundle:%s, pidFile: %s}\n", cfg.ID, cfg.Bundle, cfg.PIDFile)
	return nil
}
