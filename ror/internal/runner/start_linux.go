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
	cmd := exec.Command("/proc/self/exe", "init", id)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	var cloneFlags uintptr
	hasUserNS := false

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

	if !hasUserNS {
		return fmt.Errorf("user namespace is required for rootless containers")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 cloneFlags,
		GidMappingsEnableSetgroups: false,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container init process: %w", err)
	}

	pid := cmd.Process.Pid
	log.Printf("Container process started with PID: %d", pid)

	if err := writeIDMappings(pid, spec); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to write ID mappings: %w", err)
	}

	pidFilePath := filepath.Join(containerStatePath, "pid")

	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	return cmd.Wait()
}

func writeIDMappings(pid int, spec specs.Spec) error {
	uidMapPath := fmt.Sprintf("/proc/%d/uid_map", pid)
	uidMapContent := ""

	for _, mapping := range spec.Linux.UIDMappings {
		uidMapContent += fmt.Sprintf("%d %d %d\n", mapping.ContainerID, mapping.HostID, mapping.Size)
	}

	log.Printf("Writing UID mappings to %s: %s", uidMapPath, uidMapContent)
	if err := os.WriteFile(uidMapPath, []byte(uidMapContent), 0644); err != nil {
		return fmt.Errorf("failed to write uid_map: %w", err)
	}

	setGroupsPath := fmt.Sprintf("/proc/%d/setgroups", pid)
	if err := os.WriteFile(setGroupsPath, []byte("deny"), 0644); err != nil {
		return fmt.Errorf("failed to write setgroups: %w", err)
	}

	gidMapPath := fmt.Sprintf("/proc/%d/gid_map", pid)
	gidMapContent := ""

	for _, mapping := range spec.Linux.GIDMappings {
		gidMapContent += fmt.Sprintf("%d %d %d\n", mapping.ContainerID, mapping.HostID, mapping.Size)
	}

	log.Printf("Writing GID mappings to %s: %s", gidMapPath, gidMapContent)
	if err := os.WriteFile(gidMapPath, []byte(gidMapContent), 0644); err != nil {
		return fmt.Errorf("failed to write gid_map: %w", err)
	}

	return nil
}
