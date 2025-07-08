//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func initContainer(id string) error {
	containerStatePath := filepath.Join(defaultBasePath, id)
	// Load the blueprint (config.json)
	configJSON, err := os.ReadFile(filepath.Join(containerStatePath, "config.json"))
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	// unmarshall config into spec struct
	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshall bundle into OCI spec: %w", err)
	}

	log.Printf("Successfully loaded spec for container '%s'. Starting...", id)

	if err := syscall.Sethostname([]byte(spec.Hostname)); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	if err := pivotRoot(spec.Root.Path); err != nil {
		return fmt.Errorf("failed to pivot root: %w", err)
	}
	syscall.Mount(spec.Root.Path, spec.Root.Path, "bind", syscall.MS_BIND|syscall.MS_REC, "")
	// what is the old root?
	syscall.Mkdir("./rootfs/.pivot_root", 0755)

	syscall.PivotRoot(spec.Root.Path, "./rootfs/.pivot_root")
	os.Chdir("/")

	// how do I unmount the old root?
	syscall.Unmount("/.pivot_root", syscall.MNT_DETACH)

	for _, mount := range spec.Mounts {
		mountTarget := filepath.Join(spec.Root.Path, mount.Destination)
		mountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

		syscall.Mount(mount.Source, mountTarget, mount.Type, uintptr(mountFlags), "0")
	}

	// execute sh command here?
	// run command here?

	return nil
}

func pivotRoot(newRoot string) error {
	// mark new root as a private mount to prevent mount events from propagating to host
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to make root private: %w", err)
	}

	// bind mount new root to itself
	if err := syscall.Mount(newRoot, newRoot, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount new root: %w", err)
	}

	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0755); err != nil {
		return fmt.Errorf("failed to create pivot dir: %w", err)
	}

	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return fmt.Errorf("pivot_root faile: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root failed: %w", err)
	}

	oldRoot := "/.pivot_root"
	if err := syscall.Unmount(oldRoot, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("failed to unmount old root: %w", err)
	}

	return os.RemoveAll(oldRoot)
}
