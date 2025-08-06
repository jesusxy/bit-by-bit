//go:build linux

package runner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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

	if err := syscall.Sethostname([]byte("container")); err != nil {
		log.Printf("[WARN] couldnt set hostname: %v", err)
	}

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

	log.Printf("[ROOTLESS LIMITATION] Filesystem isolation not available - changing working dir only")
	log.Printf("Container processes will run in : %s", absRootFsPath)

	if err := os.Chdir(absRootFsPath); err != nil {
		return fmt.Errorf("chdir to / failed: %w", err)
	}

	for _, env := range spec.Process.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			os.Setenv(parts[0], parts[1])
		}
	}

	containerPath := fmt.Sprintf("%s/bin:%s/usr/bin:%s/sbin:%s/usr/sbin",
		absRootFsPath, absRootFsPath, absRootFsPath, absRootFsPath)
	os.Setenv("PATH", containerPath)
	os.Setenv("PWD", "/")

	command := spec.Process.Args[0]

	searchPaths := []string{
		filepath.Join("bin", command),
		filepath.Join("usr/bin", command),
		filepath.Join("sbin", command),
		filepath.Join("usr/sbin", command),
		command,
	}

	execPath := ""
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			execPath = path
			break
		}
	}

	if execPath == "" {
		return fmt.Errorf("command '%s' not found in container", command)
	}

	if !filepath.IsAbs(execPath) {
		execPath = filepath.Join(absRootFsPath, execPath)
	}

	log.Printf("Exec-ing command: %s with args %v", command, spec.Process.Args)
	spec.Process.Args[0] = execPath
	return syscall.Exec(execPath, spec.Process.Args, os.Environ())
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
