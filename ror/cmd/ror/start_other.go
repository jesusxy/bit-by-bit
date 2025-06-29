//go:build !linux

package main

import "fmt"

func startContainer(id string) error {
	return fmt.Errorf("cannot start container on non-linux OS")
}
