package runner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) CreateContainer(cfg ContainerConfig) error {
	if cfg.ID == "" {
		return fmt.Errorf("container id required")
	}

	containerStatePath := filepath.Join(cfg.BasePath, cfg.ID)
	bundleConfigPath := filepath.Join(cfg.Bundle, "config.json")

	if err := os.MkdirAll(containerStatePath, 0755); err != nil {
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

	newConfigPath := filepath.Join(containerStatePath, "config.json")
	if err := os.WriteFile(newConfigPath, configJSON, 0644); err != nil {
		return fmt.Errorf("failed to write config to state directory: %w", err)
	}

	log.Printf("Creating container {id:%s, bundle:%s, pidFile: %s}\n", cfg.ID, cfg.Bundle, cfg.PIDFile)
	return nil
}
