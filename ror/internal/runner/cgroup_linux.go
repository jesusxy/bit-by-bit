//go:build linux

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func selfCgroupV2() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" { // unified hierarchy
			return filepath.Join("/sys/fs/cgroup", parts[2]), nil
		}
	}
	return "", fmt.Errorf("no cgroup‑v2 entry found in /proc/self/cgroup")
}

func writeFile(path, val string) error {
	return os.WriteFile(path, []byte(val), 0644)
}

func enableControllers(dir string) error {
	return writeFile(filepath.Join(dir, "cgroup_subtree.control"), "+memory +pids +cpu")
}

func addPid(cgroup string, hostPid int) error {
	return writeFile(filepath.Join(cgroup, "cgroup.procs"), strconv.Itoa(hostPid))
}
