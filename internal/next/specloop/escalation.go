package specloop

import "github.com/danabrams/gromit/internal/next/runstore"

// TaskIntersectsEscalated returns true if the task addresses any of the provided escalated failures.
func TaskIntersectsEscalated(task *runstore.Task, escalatedFailures []string) bool {
	if task == nil || len(escalatedFailures) == 0 {
		return false
	}
	if len(task.FailuresAddressed) == 0 {
		return false
	}
	for _, failure := range escalatedFailures {
		for _, target := range task.FailuresAddressed {
			if failure == target {
				return true
			}
		}
	}
	return false
}
