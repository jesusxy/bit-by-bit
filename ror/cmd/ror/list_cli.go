package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jesuskeys/bit-by-bit/ror/internal/runner"
	"github.com/urfave/cli/v3"
)

func newListCmd(r *runner.Runner) *cli.Command {
	return &cli.Command{
		Name:    "list",
		Usage:   "List all containers",
		Aliases: []string{"ls"},
		Action: func(ctx context.Context, c *cli.Command) error {
			containers, err := r.ListContainers()

			if err != nil {
				return fmt.Errorf("failed to list containers : %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
			fmt.Fprintf(w, "ID\tSTATUS\tPID\tBUNDLE\n")

			for _, c := range containers {
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", c.ID, c.Status, c.PID, c.Bundle)
			}

			return w.Flush()
		},
	}
}
