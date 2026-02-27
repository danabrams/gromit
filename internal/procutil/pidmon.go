package procutil

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	// DefaultPIDCurrentPath is the cgroup v2 path for current PID count.
	DefaultPIDCurrentPath = "/sys/fs/cgroup/pids.current"
	// DefaultPIDMaxPath is the cgroup v2 path for PID limit.
	DefaultPIDMaxPath = "/sys/fs/cgroup/pids.max"
)

// PIDPressure reads cgroup PID usage and returns current count, max limit, and any error.
// Returns (0, 0, err) if cgroup files are not readable (non-cgroup environment).
// A max of 0 means unlimited (the cgroup file contained "max").
func PIDPressure() (current int, max int, err error) {
	return pidPressureFrom(DefaultPIDCurrentPath, DefaultPIDMaxPath)
}

// pidPressureFrom reads PID usage from the given file paths, enabling testability
// without real cgroup files.
func pidPressureFrom(currentPath, maxPath string) (current int, max int, err error) {
	curData, err := os.ReadFile(currentPath)
	if err != nil {
		return 0, 0, fmt.Errorf("reading pids.current: %w", err)
	}
	current, err = strconv.Atoi(strings.TrimSpace(string(curData)))
	if err != nil {
		return 0, 0, fmt.Errorf("parsing pids.current: %w", err)
	}

	maxData, err := os.ReadFile(maxPath)
	if err != nil {
		return 0, 0, fmt.Errorf("reading pids.max: %w", err)
	}
	maxStr := strings.TrimSpace(string(maxData))
	if maxStr == "max" {
		return current, 0, nil
	}
	max, err = strconv.Atoi(maxStr)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing pids.max: %w", err)
	}

	return current, max, nil
}
