//go:build linux

package runner

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

func (r *Runner) StartContainer(id string) error {
	log.Printf("[RUNNING] container: %v\n", id)
	// locate the container state via id
	containerStatePath := filepath.Join(r.BasePath, id)

	if _, err := os.Stat(containerStatePath); os.IsNotExist(err) {
		// If we get an "IsNotExist" error, it means the directory isn't there.
		return fmt.Errorf("container with id '%s' does not exist", id)
	}

	configJSON, err := os.ReadFile(filepath.Join(containerStatePath, "config.json"))
	if err != nil {
		return fmt.Errorf("failed to read container config: %w", err)
	}

	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshal container spec: %w", err)
	}
	// --- prepare the command to run ---
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not find executable path: %w", err)
	}
	cmd := exec.Command(executable, "init", id)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	var cloneFlags uintptr
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case "user":
			cloneFlags |= syscall.CLONE_NEWUSER
		case "pid":
			cloneFlags |= syscall.CLONE_NEWPID
		case "mount":
			cloneFlags |= syscall.CLONE_NEWNS
		case "uts":
			cloneFlags |= syscall.CLONE_NEWUTS
		case "ipc":
			cloneFlags |= syscall.CLONE_NEWIPC
		case "network":
			cloneFlags |= syscall.CLONE_NEWNET
		}
	}

	if cloneFlags&syscall.CLONE_NEWUSER == 0 {
		return fmt.Errorf("user namespace is required for rootless containers")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 cloneFlags,
		UidMappings:                []syscall.SysProcIDMap{},
		GidMappings:                []syscall.SysProcIDMap{},
		GidMappingsEnableSetgroups: false,
	}

	for _, mapping := range spec.Linux.UIDMappings {
		cmd.SysProcAttr.UidMappings = append(cmd.SysProcAttr.UidMappings, syscall.SysProcIDMap{
			ContainerID: int(mapping.ContainerID),
			HostID:      int(mapping.HostID),
			Size:        int(mapping.Size),
		})
	}

	for _, mapping := range spec.Linux.GIDMappings {
		cmd.SysProcAttr.GidMappings = append(cmd.SysProcAttr.GidMappings, syscall.SysProcIDMap{
			ContainerID: int(mapping.ContainerID),
			HostID:      int(mapping.HostID),
			Size:        int(mapping.Size),
		})
	}

	if len(cmd.SysProcAttr.UidMappings) == 0 {
		return fmt.Errorf("at leas one UID mapping is required for rootless containers")
	}

	if len(cmd.SysProcAttr.GidMappings) == 0 {
		return fmt.Errorf("at least one GID mapping is required for rootless containers")
	}

	log.Printf("Starting container with namespaces: clone_flags=0x%x", cloneFlags)
	log.Printf("UID mappings: %+v", cmd.SysProcAttr.UidMappings)
	log.Printf("GID mappings: %+v", cmd.SysProcAttr.GidMappings)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container init process: %w", err)
	}

	pid := cmd.Process.Pid
	pidFilePath := filepath.Join(containerStatePath, "pid")

	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	return cmd.Wait()
}
