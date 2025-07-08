package main

import (
	"context"
	"log"
	"os"

	"github.com/jesuskeys/bit-by-bit/ror/internal/runner"
	"github.com/urfave/cli/v3"
)

var Version = "dev"

const defaultBasePath = "./run/ror"

func main() {
	runner, err := runner.New(defaultBasePath)
	if err != nil {
		log.Fatal(err)
	}

	root := &cli.Command{
		Name:    "ror",
		Usage:   "Rootless OCI runner",
		Version: Version,
		Commands: []*cli.Command{
			newCreateCmd(runner),
			newStartCmd(runner),
			newDeleteCmd(runner),
			newInitCmd(runner),
		},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
