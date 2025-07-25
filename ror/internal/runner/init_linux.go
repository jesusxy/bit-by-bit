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

	if err := syscall.Sethostname([]byte(spec.Hostname)); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	absRootFsPath := filepath.Join("/home/ubuntu/busybox-bundle", spec.Root.Path)

	if err := pivotRoot(absRootFsPath); err != nil {
		return fmt.Errorf("failed to pivot root: %w", err)
	}

	if err := mountFs(spec.Mounts); err != nil {
		return fmt.Errorf("failed to mount special filesystems: %w", err)
	}

	command, err := exec.LookPath(spec.Process.Args[0])
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH: %w", spec.Process.Args[0], err)
	}

	log.Printf("Exec-ing command: %s with args %v and env %v", command, spec.Process.Args, spec.Process.Env)
	return syscall.Exec(command, spec.Process.Args, spec.Process.Env)
}

func pivotRoot(newRoot string) error {
	// mark new root as a private mount to prevent mount events from propagating to host
	if err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to make root private: %w", err)
	}

	// bind mount new root to itself
	if err := syscall.Mount(newRoot, newRoot, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount new root: %w", err)
	}

	pivotDir := filepath.Join(newRoot, ".pivot_root")
	if err := os.Mkdir(pivotDir, 0755); err != nil {
		return fmt.Errorf("failed to create pivot dir: %w", err)
	}

	if err := syscall.PivotRoot(newRoot, pivotDir); err != nil {
		return fmt.Errorf("pivot_root faile: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to new root failed: %w", err)
	}

	oldRoot := "/.pivot_root"
	if err := syscall.Unmount(oldRoot, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("failed to unmount old root: %w", err)
	}

	return os.RemoveAll(oldRoot)
}

func mountFs(mounts []specs.Mount) error {
	optionsMap := map[string]uintptr{
		"ro":          syscall.MS_RDONLY,
		"nosuid":      syscall.MS_NOSUID,
		"noexec":      syscall.MS_NOEXEC,
		"nodev":       syscall.MS_NODEV,
		"rbind":       syscall.MS_BIND | syscall.MS_REC,
		"bind":        syscall.MS_BIND,
		"strictatime": syscall.MS_STRICTATIME,
		"relatime": syscall.MS_RELATIME,
	}

	for _, mount := range mounts {
		if mount.Destination != "/" {
			if err := os.MkdirAll(mount.Destination, 0755); err != nil {
				return fmt.Errorf("failed to create mount dest %s:%w", mount.Destination, err)
			}
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

		if mount.Destination == "/sys/fs/cgroup" {
			log.Printf("Handling cgroup mount with a recursive bind mount to avoid EBUSY")
			err := syscall.Mount("/sys/fs/cgroup", "/sys/fs/cgroup", "", mountFlags | syscall.MS_BIND | syscall.MS_REC, "")
			if err != nil {
				return fmt.Errorf("failed to bind mount cgrup fs -> %s:%w", mount.Destination, err)
			}

			continue
		}

		data := strings.Join(dataOptions, ",")

		log.Printf("Mounting %s to %s, type: %s, flags: %d, data: %s", mount.Source, mount.Destination, mount.Type, mountFlags, data)

		if err := syscall.Mount(mount.Source, mount.Destination, mount.Type, mountFlags, data); err != nil {
			if mount.Destination == "/sys" {
				log.Printf("optional /sys mount failed: %v", err)
				continue
			}

			return fmt.Errorf("failed to mount -> %s:%w", mount.Destination, err)
		}
	}

	return nil
}
