//go:build linux

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// would I pass the processId here as an arg?
func (r *Runner) DeleteContainer(id string) error {
	containerStatePath := filepath.Join(r.BasePath, id)
	pidFilePath := filepath.Join(containerStatePath, "pid")

	// i think im missing the pid in the path here when reading from the containers state dir
	content, err := os.ReadFile(pidFilePath)
	if err != nil {
		return fmt.Errorf("failed to read contents from pid file: %w", err)
	}

	pid, err := strconv.Atoi(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse pid: %w", err)
	}

	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		fmt.Printf("failed to kill process %d: %v\n", pid, err)
	}

	return os.RemoveAll(containerStatePath)
}
