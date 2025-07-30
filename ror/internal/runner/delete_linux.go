//go:build linux

package runner

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// would I pass the processId here as an arg?
func (r *Runner) DeleteContainer(id string) error {
	containerStatePath := filepath.Join(r.BasePath, id)

	if _, err := os.Stat(containerStatePath); err != nil {
		return fmt.Errorf("container %s does not exist", id)
	}

	if err := r.terminateContainerProcess(containerStatePath); err != nil {
		log.Printf("Failed to terminate process for container %s: %v", id, err)
	}

	if err := os.RemoveAll(containerStatePath); err != nil {
		return fmt.Errorf("failed to remove container directory: %w", err)
	}
	fmt.Printf("Container %s deleted\n", id)

	return nil
}

func (r *Runner) terminateContainerProcess(containerStatePath string) error {
	pidFilePath := filepath.Join(containerStatePath, "pid")

	// i think im missing the pid in the path here when reading from the containers state dir
	content, err := os.ReadFile(pidFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("failed to read contents from pid file: %w", err)
	}

	pid, err := strconv.Atoi(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse pid: %w", err)
	}

	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil // process doesnt exist
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("failed to kill process %d: %w", pid, err)
		}
	}

	log.Printf("Terminated process %d", pid)
	return nil
}
