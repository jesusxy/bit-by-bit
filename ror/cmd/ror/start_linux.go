//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func startContainer(id string) error {
	// locate the container state via id
	containerState := filepath.Join(defaultBasePath, id)

	if _, err := os.Stat(containerState); os.IsNotExist(err) {
		// If we get an "IsNotExist" error, it means the directory isn't there.
		return fmt.Errorf("container with id '%s' does not exist", id)
	}

	// --- prepare the command to run ---

	log.Printf("[RUNNING] contianer: %v\n", id)

	cmd := exec.Command("/proc/self/exe", "init", id)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
	}

	return cmd.Run()
}
