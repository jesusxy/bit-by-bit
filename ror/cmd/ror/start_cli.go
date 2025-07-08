package main

import (
	"context"
	"fmt"

	"github.com/jesuskeys/bit-by-bit/ror/internal/runner"
	"github.com/urfave/cli/v3"
)

func newStartCmd(r *runner.Runner) *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "Start a created container",
		ArgsUsage: "<id>",
		Action: func(c context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() < 1 {
				return cli.Exit("container id required", 1)
			}

			fmt.Printf("[Starting Container::] %s\n", cmd.Args().First())
			id := cmd.Args().First()

			return r.StartContainer(id)
		},
	}
}
