//go:build linux

package runner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) InitContainer(id string) error {
	containerStatePath := filepath.Join(r.BasePath, id)
	// Load the blueprint (config.json)
	configJSON, err := os.ReadFile(filepath.Join(containerStatePath, "config.json"))
	if err != nil {
		return fmt.Errorf("failed to read bundle config: %w", err)
	}

	// unmarshall config into spec struct
	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshall bundle into OCI spec: %w", err)
	}

	log.Printf("Successfully loaded spec for container '%s'. Starting...", id)
	// [ROR ROOTLESS] The following operations require CAP_SYS_ADMIN, which a rootless
	// container does not have.
	// if err := syscall.Sethostname([]byte(spec.Hostname)); err != nil {
	// 	return fmt.Errorf("failed to set hostname: %w", err)
	// }

	absRootFsPath := filepath.Join("/home/ubuntu/busybox-bundle", spec.Root.Path)

	if err := pivotRoot(absRootFsPath); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	mountFs(spec.Mounts)

	command, err := exec.LookPath(spec.Process.Args[0])
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH: %w", spec.Process.Args[0], err)
	}

	log.Printf("Exec-ing command: %s with args %v and env %v", command, spec.Process.Args, spec.Process.Env)
	return syscall.Exec(command, spec.Process.Args, spec.Process.Env)
}

func pivotRoot(newRoot string) error {
	if err := syscall.Mount(newRoot, newRoot, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount new root: %w", err)
	}

	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.MkdirAll(pivotDir, 0755); err != nil {
		return fmt.Errorf("failed to create pivot dir: %w", err)
	}

	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root failed: %w", err)
	}

	oldRoot := "/.pivot_root"
	if err := syscall.Unmount(oldRoot, syscall.MNT_DETACH); err != nil {
		log.Printf("[WARNING] failed to unmount old root: %v", err)
	}

	if err := os.RemoveAll(oldRoot); err != nil {
		log.Printf("[WARNING] failed to remove old root dir: %v", err)
	}

	return nil
}

func mountFs(mounts []specs.Mount) {
	optionsMap := map[string]uintptr{
		"ro":          syscall.MS_RDONLY,
		"nosuid":      syscall.MS_NOSUID,
		"noexec":      syscall.MS_NOEXEC,
		"nodev":       syscall.MS_NODEV,
		"rbind":       syscall.MS_BIND | syscall.MS_REC,
		"bind":        syscall.MS_BIND,
		"strictatime": syscall.MS_STRICTATIME,
		"relatime":    syscall.MS_RELATIME,
	}

	for _, mount := range mounts {
		if err := os.MkdirAll(mount.Destination, 0755); err != nil {
			log.Printf("[WARNING]: could not create mount destination %s: %v", mount.Destination, err)
			continue
		}

		var mountFlags uintptr
		var dataOptions []string

		for _, opt := range mount.Options {
			if flag, exists := optionsMap[opt]; exists {
				mountFlags |= flag
			} else {
				dataOptions = append(dataOptions, opt)
			}
		}

		data := strings.Join(dataOptions, ",")

		log.Printf("Mounting %s to %s, type: %s, flags: %d, data: %s", mount.Source, mount.Destination, mount.Type, mountFlags, data)

		if err := syscall.Mount(mount.Source, mount.Destination, mount.Type, mountFlags, data); err != nil {
			log.Printf("WARNING: could not mount %s: %v (this is expected in rootless mode)", mount.Destination, err)
		}
	}
}
