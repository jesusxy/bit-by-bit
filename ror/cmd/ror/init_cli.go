//go:build !linux

package main

import "fmt"

// This is the dummy function for non-Linux systems.
func initContainer(id string) error {
	return fmt.Errorf("init command is for internal use on linux only")
}
