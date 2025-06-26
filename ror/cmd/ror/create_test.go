package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Test that the logic correctly returns an error if the ID is missing.
func TestCreateContainerRequiresID(t *testing.T) {
	// No ID is provided in this config.
	cfg := CreateCmdConfig{Bundle: ".", PIDFile: ""}
	err := createContainer(cfg)

	if err == nil {
		t.Fatal("expected an error when container ID is missing, but got nil")
	}

	expectedErr := "container id required"
	if err.Error() != expectedErr {
		t.Fatalf("expected error '%s', but got '%s'", expectedErr, err.Error())
	}
}

func TestCreateContainerSuccess(t *testing.T) {
	tempDir := t.TempDir()

	bundleDir := filepath.Join(tempDir, "my-bundle")
	if err := os.MkdirAll(bundleDir, 0755); err != nil {
		t.Fatalf("failed to create fake bundle dir: %v", err)
	}

	fakeConfig := `{"ociVersion": "1.0.0"}`
	if err := os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(fakeConfig), 0644); err != nil {
		t.Fatalf("failed to write fake config.json: %v", err)
	}

	stateDir := filepath.Join(tempDir, "ror-state")
	cfg := CreateCmdConfig{
		ID:       "test-container-1",
		Bundle:   bundleDir,
		BasePath: stateDir,
	}

	if err := createContainer(cfg); err != nil {
		t.Fatalf("createContainer failed: %v", err)
	}

	expectedStatePath := filepath.Join(stateDir, cfg.ID)
	if _, err := os.Stat(expectedStatePath); os.IsNotExist(err) {
		t.Fatalf("expected state directory to be created at %s, but it was not", expectedStatePath)
	}

	copiedConfigPath := filepath.Join(expectedStatePath, "config.json")
	content, err := os.ReadFile(copiedConfigPath)

	if err != nil {
		t.Fatalf("failed to read copied config.json: %v", err)
	}

	if string(content) != fakeConfig {
		t.Fatalf("config.json content mismatch. got %q, want %q", string(content), fakeConfig)
	}
}
