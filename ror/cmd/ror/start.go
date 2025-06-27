package main

import (
	"context"
	"fmt"

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
	return nil
}
