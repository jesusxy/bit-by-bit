package main

import (
	"context"
	"testing"

	cli "github.com/urfave/cli/v3"
)

func TestCreateCmdRequiresId(t *testing.T) {
	cmd := newCreateCmd()
	ctx := context.Background()

	err := cmd.Run(ctx, []string{"ror"})

	if ec, ok := err.(cli.ExitCoder); !ok || ec.ExitCode() != 1 {
		t.Fatalf("expected cli.ExitCoder with code 1, got %v", err)
	}
}

func TestCreateCmdParsesFlags(t *testing.T) {
	var gotID, gotBundle string
	cmd := newCreateCmd()

	cmd.Action = func(_ context.Context, c *cli.Command) error {
		gotID = c.Args().First()
		gotBundle = c.String("bundle")
		return nil
	}

	err := cmd.Run(context.Background(), []string{"ror", "--bundle", "/tmp/busybox", "box"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotID != "box" || gotBundle != "/tmp/busybox" {
		t.Fatalf("parsed {id:%v, bundle:%s}", gotID, gotBundle)
	}
}
