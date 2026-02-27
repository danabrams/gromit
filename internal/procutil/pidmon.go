package procutil

// PIDPressure reads cgroup PID usage and returns current count, max limit, and any error.
// It dynamically resolves the process's cgroup path via /proc/self/cgroup, making it
// correct in containerized environments where the cgroup root varies.
// Returns (0, 0, err) if cgroup files are not readable (non-cgroup environment).
// A max of 0 means unlimited (the cgroup file contained "max").
func PIDPressure() (current int, max int, err error) {
	return readCgroupPIDUsage()
}
