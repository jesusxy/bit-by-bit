package main

import (
	"context"

	"github.com/jesuskeys/bit-by-bit/ror/internal/logger"
	"github.com/jesuskeys/bit-by-bit/ror/internal/runner"
	"github.com/urfave/cli/v3"
)

func newDeleteCmd(r *runner.Runner) *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete container state",
		ArgsUsage: "<id>",
		Action: func(c context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() < 1 {
				return cli.Exit("container id required", 1)
			}

			id := cmd.Args().First()
			logger.Info("Deleting container %s\n", id)
			return r.DeleteContainer(id)
		},
	}
}
