package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func newDeleteCmd() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "Delete container state",
		ArgsUsage: "<id>",
		Action: func(c context.Context, cmd *cli.Command) error {
			if cmd.Args().Len() < 1 {
				return cli.Exit("container id required", 1)
			}

			fmt.Printf("Deleting container %s\n", cmd.Args().First())
			return nil
		},
	}
}
