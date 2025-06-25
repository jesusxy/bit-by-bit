package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func newCreateCmd() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "Create an OCI container",
		ArgsUsage: "<id>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "bundle",
				Aliases: []string{"b"},
				Value:   ".",
				Usage:   "Path to the OCI bundle",
			},
			&cli.StringFlag{
				Name:  "pid-file",
				Usage: "write child pid to this file (optional)",
			},
		},
		Action: func(c context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() < 1 {
				return cli.Exit("container id required", 1)
			}

			id := cmd.Args().First()
			bundle := cmd.String("bundle")
			pid := cmd.String("pid-file")

			fmt.Printf("Creating container {id:%s, bundle:%s, pidFile: %s}\n", id, bundle, pid)
			return nil
		},
	}
}
