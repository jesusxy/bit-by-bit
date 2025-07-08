//go:build !linux

package runner

import "fmt"

// This is the dummy function for non-Linux systems.
func (r *Runner) InitContainer(id string) error {
	return fmt.Errorf("init command is for internal use on linux only")
}
