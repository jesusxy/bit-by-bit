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

func (r *Runner) InitChild(id string) error {
	log.Printf("[CHILD] Process started for container: %s", id)
	pipe := os.NewFile(3, "sync-pipe")
	buf := make([]byte, 2)
	log.Printf("[CHILD] Waiting for parent to set up ID mappings....")

	if _, err := pipe.Read(buf); err != nil {
		return fmt.Errorf("failed to sync with parent: %w", err)
	}
	pipe.Close()

	log.Printf("[CHILD] parent signaled. Continuing intitialization.")

	if uid := os.Getuid(); uid != 0 {
		return fmt.Errorf("expected to be root in new user namespace, but UID is %d", uid)
	}

	log.Printf("[CHILD] verified UID is 0 in the new user namespace.")

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

	absRootFsPath := filepath.Join("/home/ubuntu/busybox-bundle", spec.Root.Path)
	log.Printf("Changing root to: %s", absRootFsPath)

	// For rootless containers, we must use pivot_root instead of chroot
	// First, we need to bind mount the new root to itself to make it a mount point
	if err := syscall.Mount(absRootFsPath, absRootFsPath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount new root: %w", err)
	}

	// Create a temporary directory for the old root
	oldRoot := filepath.Join(absRootFsPath, ".pivot_root")
	if err := os.MkdirAll(oldRoot, 0755); err != nil {
		return fmt.Errorf("failed to create old root dir: %w", err)
	}

	// Change to the new root before pivot_root
	if err := os.Chdir(absRootFsPath); err != nil {
		return fmt.Errorf("failed to chdir to new root: %w", err)
	}

	// Perform pivot_root
	if err := syscall.PivotRoot(".", ".pivot_root"); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	// Change to the new root
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir to / failed: %w", err)
	}

	// Unmount and remove the old root
	if err := syscall.Unmount("/.pivot_root", syscall.MNT_DETACH); err != nil {
		log.Printf("[WARNING] failed to unmount old root: %v", err)
	}
	if err := os.Remove("/.pivot_root"); err != nil {
		log.Printf("[WARNING] failed to remove old root dir: %v", err)
	}

	log.Printf("Successfully changed root using pivot_root")

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
