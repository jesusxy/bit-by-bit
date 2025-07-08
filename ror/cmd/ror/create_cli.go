package main

import (
	"context"

	"github.com/jesuskeys/ror/internal/runner"
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
			cfg.BasePath = defaultBasePath
			return runner.CreateContainer(cfg)
		},
	}
}
