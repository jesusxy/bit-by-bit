//go:build !linux

package runner

import "fmt"

func (r *Runner) StartContainer(id string) error {
	return fmt.Errorf("cannot start container on non-linux OS")
}
