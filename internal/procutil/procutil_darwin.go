//go:build darwin

package procutil

import (
	"os/exec"
	"strconv"
	"strings"
)

// collectDescendants recursively discovers all descendant PIDs of the given
// process on macOS using pgrep -P <pid>.
func collectDescendants(pid int) []int {
	var pids []int
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns non-zero exit code if no matches found; treat as empty list
		return nil
	}

	// Parse the output: each line contains a child PID
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		childPid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, childPid)
		pids = append(pids, collectDescendants(childPid)...)
	}
	return pids
}
