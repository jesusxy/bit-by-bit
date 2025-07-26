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
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) InitContainer(id string) error {
	time.Sleep(100 * time.Millisecond)
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
	log.Printf("Init process UID: %d, GID: %d", os.Getuid(), os.Getgid())
	log.Printf("Init process EUID: %d, EGID: %d", os.Geteuid(), os.Getegid())

	if os.Getuid() != 0 {
		return fmt.Errorf("not root in user namespace, UID is %d", os.Geteuid())
	}

	absRootFsPath := filepath.Join("/home/ubuntu/busybox-bundle", spec.Root.Path)
	log.Printf("Changing root to: %s", absRootFsPath)

	if err := syscall.Chroot(absRootFsPath); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to / failed: %w", err)
	}

	log.Printf("Successfully changed root")

	mountFs(spec.Mounts)

	for _, env := range spec.Process.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}

	command, err := exec.LookPath(spec.Process.Args[0])
	if err != nil {
		return fmt.Errorf("command '%s' not found in PATH: %w", spec.Process.Args[0], err)
	}

	log.Printf("Exec-ing command: %s with args %v and env %v", command, spec.Process.Args, spec.Process.Env)
	return syscall.Exec(command, spec.Process.Args, spec.Process.Env)
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
		if isRootlessIncompatible(mount) {
			log.Printf("[INFO] Skipping mount %s (incompatible with rootless)", mount.Destination)
			continue
		}

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
			log.Printf("[INFO] Mount %s failed: %v (this is often expected in rootless mode)", mount.Destination, err)
		} else {
			log.Printf("[INFO] Successfully mounted %s", mount.Destination)
		}
	}
}

func isRootlessIncompatible(mount specs.Mount) bool {
	incompatibleTypes := map[string]bool{
		"sysfs":   true,
		"cgroup":  true,
		"cgroup2": true,
	}

	incompatiblePaths := []string{
		"/sys",
		"/sys/fs/cgroup",
	}

	if incompatibleTypes[mount.Type] {
		return true
	}

	for _, path := range incompatiblePaths {
		if mount.Destination == path || strings.HasPrefix(mount.Destination, path+"/") {
			return true
		}
	}

	return false
}
