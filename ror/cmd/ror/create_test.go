package main

import (
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

// Test the successful logic path.
func TestCreateContainer(t *testing.T) {
	// Provide a valid config.
	cfg := CreateCmdConfig{
		ID:      "my-test-box",
		Bundle:  "/tmp/busybox",
		PIDFile: "/var/run/test.pid",
	}

	err := createContainer(cfg)

	// In a real test, you would check that the container was
	// actually created. For now, we just check for no errors.
	if err != nil {
		t.Fatalf("expected no error, but got: %v", err)
	}
}
