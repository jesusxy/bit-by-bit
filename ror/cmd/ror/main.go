package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

const version = "v0.0.1-dev"

func main() {
	root := &cli.Command{
		Name:  "ror",
		Usage: "Rootless OCI runner",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "Create an OCI container",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "bundle",
						Aliases: []string{"b"},
						Value:   ".",
						Usage:   "Path to the OCI bundle",
					},
					&cli.StringFlag{
						Name:  "pid-file",
						Value: "",
						Usage: "write child pid to this file",
					},
				},
				Action: func(c context.Context, cmd *cli.Command) error {
					if cmd.Args().Len() < 1 {
						return cli.Exit("container id required", 1)
					}

					id := cmd.Args().First()
					bundle := cmd.String("bundle")
					pid := cmd.String("pid-file")

					fmt.Printf("Creating container { Id: %s, bundle: %s, pid: %s\n", id, bundle, pid)
					return nil
				},
			},
			{
				Name:  "start",
				Usage: "Start a created container",
				Action: func(c context.Context, cmd *cli.Command) error {
					if cmd.Args().Len() < 1 {
						return cli.Exit("container id required", 1)
					}

					fmt.Printf("Running container %s\n", cmd.Args().First())

					return nil
				},
			},
			{
				Name:  "delete",
				Usage: "Delete container state",
				Action: func(c context.Context, cmd *cli.Command) error {
					if cmd.Args().Len() < 1 {
						return cli.Exit("container id required", 1)
					}

					fmt.Printf("Deleting container %s\n", cmd.Args().First())
					return nil
				},
			},
		},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
