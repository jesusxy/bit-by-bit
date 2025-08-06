package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jesuskeys/bit-by-bit/ror/internal/runner"
	"github.com/urfave/cli/v3"
)

var Version = "dev"

const defaultBasePath = "./run/ror"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		runner, err := runner.New(defaultBasePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[child]: failed to create runner %v\n", err)
			os.Exit(1)
		}

		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "[child]: missing container ID")
			os.Exit(1)
		}

		id := os.Args[2]

		if err := runner.InitChild(id); err != nil {
			fmt.Fprintf(os.Stderr, "[child]: init failed: %v\n", err)
			os.Exit(1)
		}

		return
	}

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
		},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
