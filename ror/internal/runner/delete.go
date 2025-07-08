//go:build !linux

package runner

import "fmt"

func (r *Runner) DeleteContainer(id string) error {
	return fmt.Errorf("cannot delete container on non-linux OS")
}
