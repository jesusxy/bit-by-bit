//go:build linux

package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jesuskeys/bit-by-bit/ror/internal/constants"
	"github.com/jesuskeys/bit-by-bit/ror/internal/logger"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) InitChild(id string) error {
	logger.ChildWithID(id, "Process started for container: %s")
	pipe := os.NewFile(3, "sync-pipe")
	buf := make([]byte, 2)
	logger.Child("Waiting for parent to set up ID mappings...")

	if _, err := pipe.Read(buf); err != nil {
		return fmt.Errorf("failed to sync with parent: %w", err)
	}
	pipe.Close()

	logger.Child("parent signaled. Continuing initialization.")

	if uid := os.Getuid(); uid != 0 {
		return fmt.Errorf("expected to be root in new user namespace, but UID is %d", uid)
	}

	logger.Child("verified UID is 0 in the new user namespace.")

	if err := syscall.Sethostname([]byte("container")); err != nil {
		logger.Warn("couldnt set hostname: %v", err)
	}

	containerStatePath := filepath.Join(r.BasePath, id)

	// Load the blueprint (config.json)
	configJSON, err := os.ReadFile(filepath.Join(containerStatePath, constants.ConfigFileName))
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	// unmarshall config into spec struct
	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshall bundle into OCI spec: %w", err)
	}

	absRootFsPath := filepath.Join("/home/ubuntu/busybox-bundle", spec.Root.Path)
	logger.Info("Changing root to: %s", absRootFsPath)

	logger.Info("[ROOTLESS LIMITATION] Filesystem isolation not available - changing working dir only")
	logger.Info("Container process will run in: %s", absRootFsPath)

	if err := os.Chdir(absRootFsPath); err != nil {
		return fmt.Errorf("chdir to / failed: %w", err)
	}

	for _, env := range spec.Process.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}

	containerPath := fmt.Sprintf("%s/bin:%s/usr/bin:%s/sbin:%s/usr/sbin",
		absRootFsPath, absRootFsPath, absRootFsPath, absRootFsPath)
	os.Setenv("PATH", containerPath)
	os.Setenv("PWD", "/")

	command := spec.Process.Args[0]

	searchPaths := []string{
		filepath.Join("bin", command),
		filepath.Join("usr/bin", command),
		filepath.Join("sbin", command),
		filepath.Join("usr/sbin", command),
		command,
	}

	execPath := ""
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			execPath = path
			break
		}
	}

	if execPath == "" {
		return fmt.Errorf("command '%s' not found in container", command)
	}

	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(absRootFsPath, execPath)
	}

	logger.Info("Exec-ing command %s with args %v", command, spec.Process.Args)
	spec.Process.Args[0] = execPath
	return syscall.Exec(execPath, spec.Process.Args, os.Environ())
}
