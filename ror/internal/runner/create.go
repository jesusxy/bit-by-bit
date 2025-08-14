package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jesuskeys/bit-by-bit/ror/internal/constants"
	"github.com/jesuskeys/bit-by-bit/ror/internal/logger"
	"github.com/jesuskeys/bit-by-bit/ror/internal/types"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) CreateContainer(cfg types.ContainerConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("container id required")
	}

	containerStatePath := filepath.Join(cfg.BasePath, cfg.ID)
	bundleConfigPath := filepath.Join(cfg.Bundle, constants.ConfigFileName)

	if err := os.MkdirAll(containerStatePath, constants.DefaultDirPermissions); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	configJSON, err := os.ReadFile(bundleConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	var spec specs.Spec

	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("bundle config.json is not a valid OCI spec: %w", err)
	}

	configureForRootless(&spec)

	updatedConfigJSON, err := json.MarshalIndent(spec, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal updated spec: %w", err)
	}

	newConfigPath := filepath.Join(containerStatePath, constants.ConfigFileName)
	if err := os.WriteFile(newConfigPath, updatedConfigJSON, constants.DefaultFilePermissions); err != nil {
		return fmt.Errorf("failed to write config to state directory: %w", err)
	}

	absBundlePath, err := filepath.Abs(cfg.Bundle)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for bundle: %w", err)
	}

	bundlePathFile := filepath.Join(containerStatePath, constants.BundlePathFileName)
	if err := os.WriteFile(bundlePathFile, []byte(absBundlePath), constants.DefaultFilePermissions); err != nil {
		return fmt.Errorf("failed to write bundle path file: %w", err)
	}

	logger.Info("Creating container {id: %s, bundle: %s, pidFile: %s}\n", cfg.ID, cfg.Bundle, cfg.PIDFile)
	return nil
}

func configureForRootless(spec *specs.Spec) {
	spec.Linux.Namespaces = append(spec.Linux.Namespaces, specs.LinuxNamespace{
		Type: specs.UserNamespace,
	})

	spec.Linux.UIDMappings = []specs.LinuxIDMapping{
		{
			HostID:      uint32(os.Getuid()),
			ContainerID: 0,
			Size:        1,
		},
	}

	spec.Linux.GIDMappings = []specs.LinuxIDMapping{
		{
			HostID:      uint32(os.Getgid()),
			ContainerID: 0,
			Size:        1,
		},
	}

	spec.Process.NoNewPrivileges = false
}
