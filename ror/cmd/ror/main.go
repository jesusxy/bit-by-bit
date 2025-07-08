package main

import (
	"context"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

var Version = "dev"

const defaultBasePath = "./run/ror"

func main() {
	root := &cli.Command{
		Name:    "ror",
		Usage:   "Rootless OCI runner",
		Version: Version,
		Commands: []*cli.Command{
			newCreateCmd(),
			newStartCmd(),
			newDeleteCmd(),
			newInitCmd(),
		},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
