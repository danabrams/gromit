//go:build linux

package procutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// collectDescendants recursively walks /proc/<pid>/task/*/children to
// discover all descendant PIDs of the given process on Linux.
func collectDescendants(pid int) []int {
	var pids []int
	taskDir := fmt.Sprintf("/proc/%d/task", pid)
	tasks, err := os.ReadDir(taskDir)
	if err != nil {
		return nil
	}
	for _, task := range tasks {
		childrenFile := filepath.Join(taskDir, task.Name(), "children")
		data, err := os.ReadFile(childrenFile)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			childPid, err := strconv.Atoi(field)
			if err != nil {
				continue
			}
			pids = append(pids, childPid)
			pids = append(pids, collectDescendants(childPid)...)
		}
	}
	return pids
}
