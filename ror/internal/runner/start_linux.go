//go:build linux

package runner

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) StartContainer(id string) error {
	log.Printf("[PARENT] starting container: %s\n", id)
	// locate the container state via id
	containerStatePath := filepath.Join(r.BasePath, id)

	if _, err := os.Stat(containerStatePath); os.IsNotExist(err) {
		// If we get an "IsNotExist" error, it means the directory isn't there.
		return fmt.Errorf("container with id '%s' does not exist", id)
	}

	spec, err := r.loadSpec(containerStatePath)
	if err != nil {
		return fmt.Errorf("could not load spec for container %s: %w", id, err)
	}

	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	defer pipeW.Close()

	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
		log.Printf("[WARN] couldnt get executable path, using %s", execPath)
	}
	cmd := &exec.Cmd{
		Path:       execPath,
		Args:       []string{os.Args[0], "child", id},
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		ExtraFiles: []*os.File{pipeR}, // pass read end of the pipe to child
	}

	if !hasUserNamespace(spec) {
		return fmt.Errorf("config.json must include a 'user' namspace for rootless mode")
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container init process: %w", err)
	}

	pipeR.Close()
	pid := cmd.Process.Pid
	log.Printf("[PARENT] child process started with PID: %d", pid)

	pidFilePath := filepath.Join(containerStatePath, "pid")
	if err := os.WriteFile(pidFilePath, []byte(strconv.Itoa(pid)), 0644); err != nil {
		log.Printf("[WARN] failed to write PID file: %v", err)
	}

	if err := writeIDMappings(pid, spec); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to write ID mappings: %w", err)
	}

	log.Printf("[PARENT] wrote ID mappings for PID: %d", pid)
	if _, err := pipeW.Write([]byte("go")); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to signal child: %w", err)
	}

	pipeW.Close()

	log.Printf("[PARENT] signaled child. Waiting for container to exit.")
	return cmd.Wait()
}

func writeIDMappingsDirect(pid int, spec *specs.Spec) error {
	uidMapPath := fmt.Sprintf("/proc/%d/uid_map", pid)
	uidMapContent := ""

	for _, mapping := range spec.Linux.UIDMappings {
		uidMapContent += fmt.Sprintf("%d %d %d\n", mapping.ContainerID, mapping.HostID, mapping.Size)
	}

	log.Printf("Writing UID mappings to %s: %s", uidMapPath, uidMapContent)
	if err := os.WriteFile(uidMapPath, []byte(uidMapContent), 0644); err != nil {
		return fmt.Errorf("failed to write uid_map: %w", err)
	}

	setGroupsPath := fmt.Sprintf("/proc/%d/setgroups", pid)
	if err := os.WriteFile(setGroupsPath, []byte("deny"), 0644); err != nil {
		return fmt.Errorf("failed to write setgroups: %w", err)
	}

	gidMapPath := fmt.Sprintf("/proc/%d/gid_map", pid)
	gidMapContent := ""

	for _, mapping := range spec.Linux.GIDMappings {
		gidMapContent += fmt.Sprintf("%d %d %d\n", mapping.ContainerID, mapping.HostID, mapping.Size)
	}

	log.Printf("Writing GID mappings to %s: %s", gidMapPath, gidMapContent)
	if err := os.WriteFile(gidMapPath, []byte(gidMapContent), 0644); err != nil {
		return fmt.Errorf("failed to write gid_map: %w", err)
	}

	return nil
}

func writeIDMappings(pid int, spec *specs.Spec) error {
	newuidmapPath, err := exec.LookPath("newuidmap")
	if err != nil {
		log.Printf("newuidmap not found, falling back to direct write (requires privileges)")
		return writeIDMappingsDirect(pid, spec)
	}

	newgidmapPath, err := exec.LookPath("newgidmap")
	if err != nil {
		log.Printf("newgidmap not found, falling back to direct write (requires privileges)")
		return writeIDMappingsDirect(pid, spec)
	}

	setgroupsPath := fmt.Sprintf("/proc/%d/setgroups", pid)
	if err := os.WriteFile(setgroupsPath, []byte("deny"), 0644); err != nil {
		return fmt.Errorf("failed to write setgroups: %w", err)
	}

	uidArgs := []string{strconv.Itoa(pid)}
	for _, mapping := range spec.Linux.UIDMappings {
		uidArgs = append(uidArgs,
			strconv.Itoa(int(mapping.ContainerID)),
			strconv.Itoa(int(mapping.HostID)),
			strconv.Itoa(int(mapping.Size)))
	}

	cmd := exec.Command(newuidmapPath, uidArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("newuidmap failed: %v, output: %s", err, output)
	}

	log.Printf("Successfully set UID mappings using newuidmap")

	gidArgs := []string{strconv.Itoa(pid)}
	for _, mapping := range spec.Linux.GIDMappings {
		gidArgs = append(gidArgs,
			strconv.Itoa(int(mapping.ContainerID)),
			strconv.Itoa(int(mapping.HostID)),
			strconv.Itoa(int(mapping.Size)),
		)
	}

	cmd = exec.Command(newgidmapPath, gidArgs...)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("newgidmap failed: %v, output: %w", err, output)
	}

	log.Printf("Successfully get GID mapping using newgidmap")
	return nil
}

func (r *Runner) loadSpec(containerStatePath string) (*specs.Spec, error) {
	configJSON, err := os.ReadFile(filepath.Join(containerStatePath, "config.json"))

	if err != nil {
		return nil, err
	}

	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

func hasUserNamespace(spec *specs.Spec) bool {
	if spec.Linux == nil {
		return false
	}

	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == specs.UserNamespace {
			return true
		}
	}

	return false
}
