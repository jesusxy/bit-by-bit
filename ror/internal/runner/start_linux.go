//go:build linux

package runner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/runtime-spec/specs-go"
)

func (r *Runner) StartContainer(id string) error {
	// locate the container state via id
	containerStatePath := filepath.Join(r.BasePath, id)

	if _, err := os.Stat(containerStatePath); os.IsNotExist(err) {
		// If we get an "IsNotExist" error, it means the directory isn't there.
		return fmt.Errorf("container with id '%s' does not exist", id)
	}

	// --- prepare the command to run ---

	log.Printf("[RUNNING] contianer: %v\n", id)

	currUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("unable to get current user: %w", err)
	}

	configJSON, err := os.ReadFile(filepath.Join(containerStatePath, "config.json"))

	var spec specs.Spec
	if err := json.Unmarshal(configJSON, &spec); err != nil {
		return fmt.Errorf("failed to unmarshall bundle into OCI spec: %w", err)
	}

	uidMappings, err := parseSubIDMappings(currUser.Username, "/etc/subuid", spec.Linux.UIDMappings)
	if err != nil {
		return fmt.Errorf("failed to parse subuid mappings: %w", err)
	}

	gidMappings, err := parseSubIDMappings(currUser.Username, "/etc/subgid", spec.Linux.GIDMappings)
	if err != nil {
		return fmt.Errorf("failed to parse subgid mappings: %w", err)
	}

	// --- create cgroup sandbox --- //
	cgroupPath := filepath.Join("/sys/fs/cgroup/ror/", "ror", id)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup directory %w", err)
	}

	if spec.Linux.Resources.Memory != nil && spec.Linux.Resources.Memory.Limit != nil && *spec.Linux.Resources.Memory.Limit > 0 {
		limit := *spec.Linux.Resources.Memory.Limit
		memoryLimitPath := filepath.Join(cgroupPath, "memory.max")

		if err := os.WriteFile(memoryLimitPath, []byte(strconv.FormatInt(limit, 10)), 0644); err != nil {
			return fmt.Errorf("failed to write memory limit to cgroup: %w", err)
		}
	}

	cmd := exec.Command("/proc/self/exe", "init", id)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUSER,
		UidMappings: uidMappings,
		GidMappings: gidMappings,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container init process: %w", err)
	}

	pid := cmd.Process.Pid
	pidFilePath := filepath.Join(containerStatePath, "pid")

	if err := os.WriteFile(pidFilePath, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}

	return cmd.Wait()
}

func parseSubIDMappings(username, subidFilepath string, specMapping []specs.LinuxIDMapping) ([]syscall.SysProcIDMap, error) {
	hostIDRangeStart, hostIDRangeCount, err := findSubIDRange(username, subidFilepath)
	if err != nil {
		return nil, err
	}

	var resultMappings []syscall.SysProcIDMap

	for _, specMap := range specMapping {
		if specMap.Size > uint32(hostIDRangeCount) {
			return nil, fmt.Errorf("requested mapping size %d exceeds allowed count %d", specMap.Size, hostIDRangeCount)
		}

		if specMap.ContainerID+uint32(specMap.Size) > uint32(hostIDRangeCount) {
			return nil, fmt.Errorf("requested mapping from containerID %d with size %d exceeds allowed count %d", specMap.ContainerID, specMap.Size, hostIDRangeCount)
		}

		resultMappings = append(resultMappings, syscall.SysProcIDMap{
			ContainerID: int(specMap.ContainerID),
			HostID:      hostIDRangeStart + int(specMap.ContainerID),
			Size:        int(specMap.Size),
		})
	}

	return resultMappings, nil
}

func findSubIDRange(username, filepath string) (start, count int, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		return 0, 0, err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// linux file format username:start_uid:uid_count
		/** examples:
			root:231072:512
			user1:100000:65536
			user2:165536:65536
			user3:200000:1000
		**/
		parts := strings.Split(line, ":")

		if len(parts) == 3 && parts[0] == username {
			startID, err := strconv.Atoi(parts[1])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid start_id '%s' for user %s: %w", parts[1], username, err)
			}

			count, err := strconv.Atoi(parts[2])
			if err != nil {
				return 0, 0, fmt.Errorf("invalid count: '%s' for user %s: %w", parts[2], username, err)
			}

			return startID, count, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}

	return 0, 0, fmt.Errorf("no subordinate id mapping found for user %s in %s", username, filepath)
}
