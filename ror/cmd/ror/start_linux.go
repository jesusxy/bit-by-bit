//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func startContainer(id string) error {
	// locate the container state via id
	containerState := filepath.Join(defaultBasePath, id)

	if _, err := os.Stat(containerState); os.IsNotExist(err) {
		// If we get an "IsNotExist" error, it means the directory isn't there.
		return fmt.Errorf("container with id '%s' does not exist", id)
	}

	// Load the blueprint (config.json)
	configJSON, err := os.ReadFile(filepath.Join(containerState, "config.json"))
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	// unmarshall config into spec struct
	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshall bundle into OCI spec: %w", err)
	}

	log.Printf("Successfully loaded spec for container '%s'. Starting...", id)

	// --- prepare the command to run ---

	log.Printf("[RUNNING] %v\n", os.Args[2:])
	cmd := exec.Command(os.Args[2], os.Args[3:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}

	cmd.Run()

	return nil
}
