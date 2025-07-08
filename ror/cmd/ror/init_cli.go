package main

import (
	"context"
	"fmt"

	"github.com/jesuskeys/bit-by-bit/ror/internal/runner"
	"github.com/urfave/cli/v3"
)

func newInitCmd(r *runner.Runner) *cli.Command {
	return &cli.Command{
		Name:   "init",
		Usage:  "Internal command to initialize a container (DO NOT CALL DIRECTLY)",
		Hidden: true,
		Action: func(c context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() < 1 {
				return fmt.Errorf("container id not provided for INIT")
			}
			id := cmd.Args().First()

			return r.InitContainer(id)
		},
	}
}
