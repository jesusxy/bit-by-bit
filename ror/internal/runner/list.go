package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/jesuskeys/bit-by-bit/ror/internal/constants"
)

type ContainerInfo struct {
	ID     string
	Status string
	PID    int
	Bundle string
}

func (r *Runner) ListContainers() ([]ContainerInfo, error) {
	entries, err := os.ReadDir(r.BasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	var containers []ContainerInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		containerId := entry.Name()
		info := ContainerInfo{ID: containerId}

		statePath := filepath.Join(r.BasePath, containerId)
		pidFilePath := filepath.Join(statePath, constants.PIDFileName)
		bundleFilePath := filepath.Join(statePath, constants.BundlePathFileName)

		bundlePathBytes, err := os.ReadFile(bundleFilePath)
		if err == nil {
			info.Bundle = string(bundlePathBytes)
		}

		pidData, err := os.ReadFile(pidFilePath)
		if os.IsNotExist(err) {
			info.Status = "created"
			containers = append(containers, info)
			continue
		}

		pid, err := strconv.Atoi(string(pidData))
		if err != nil {
			info.Status = "stopped"
			containers = append(containers, info)
			continue
		}
		info.PID = pid

		// check if process is actually running, by sending null signal
		process, err := os.FindProcess(pid)
		if err != nil {
			info.Status = "stopped"
			containers = append(containers, info)
			continue
		}

		err = process.Signal(syscall.Signal(0))
		if err == nil {
			info.Status = "running"
		} else {
			info.Status = "stopped"
		}
		containers = append(containers, info)
	}

	return containers, nil
}
