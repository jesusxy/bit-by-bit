//go:build linux

package runner

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func (r *Runner) StartContainer(id string) error {
	// locate the container state via id
	containerStatePath := filepath.Join(r.BasePath, id)

	if _, err := os.Stat(containerStatePath); os.IsNotExist(err) {
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
