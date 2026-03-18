package specloop

import (
	"fmt"

	"github.com/danabrams/gromit/internal/next/runstore"
)

const maxDepth = 100

// truncateLastError truncates error output to a maximum of 2000 characters.
// LastError = LastError [:2000]
func truncateLastError(lastError string) string {
	if len(lastError) > 2000 {
		return lastError[:2000]
	}
	return lastError
}

// resolveLineageRoot follows ChainIDs to find the root task of a lineage chain.
// It implements cycle protection with a maximum depth guard to prevent infinite loops.
// If a task is not in the lineage map or has empty ChainIDs, it is considered a root.
// ChainIDs[0] is always the task ID of the direct parent in the lineage chain (or self
// for the root). This invariant is established by UpdateTaskLineage.
func resolveLineageRoot(lineage map[string]runstore.TaskLineageEntry, taskID string) string {
	entry, exists := lineage[taskID]
	if !exists || len(entry.ChainIDs) == 0 {
		// No entry or empty chain means this task is the root
		return taskID
	}

	// ChainIDs[0] is the parent pointer; verify it's not a cycle
	current := entry.ChainIDs[0]
	if current == taskID {
		return taskID
	}

	// Follow the chain to ensure we find the actual root
	depth := 0
	for depth < maxDepth {
		nextEntry, exists := lineage[current]
		if !exists || len(nextEntry.ChainIDs) == 0 {
			// Reached a task with no further lineage, it's the root
			return current
		}

		next := nextEntry.ChainIDs[0]
		if next == current {
			// Self-referencing cycle, current is the root
			return current
		}

		// Check if following the chain would bring us back to the original task
		if next == taskID {
			// Cycle detected - return the current task as the root for this chain
			return current
		}

		current = next
		depth++
	}

	// Max depth reached, return current as best guess for root
	return current
}

// resolveRootViaTasksFixes walks the Fixes chain through the tasks slice to find
// the ultimate root task ID. Used when the intermediate fix tasks are not in
// the lineage map (e.g., non-root fix tasks after Issue 3 removes mirror entries).
// Returns the taskID of the root: either the first task in the Fixes chain that
// exists in the lineage map, or the last task that has no Fixes field.
func resolveRootViaTasksFixes(taskLineage map[string]runstore.TaskLineageEntry, tasks []runstore.Task, taskID string) string {
	// Build a quick lookup of taskID → Fixes from the tasks slice
	fixesMap := make(map[string]string, len(tasks))
	for _, t := range tasks {
		if t.Fixes != "" {
			fixesMap[t.TaskID] = t.Fixes
		}
	}

	current := taskID
	for depth := 0; depth < maxDepth; depth++ {
		// If current task is in the lineage, resolve it via the lineage
		if _, exists := taskLineage[current]; exists {
			return resolveLineageRoot(taskLineage, current)
		}
		// Follow Fixes chain through the tasks slice
		fixes, hasFixes := fixesMap[current]
		if !hasFixes {
			// No Fixes field: current is the root
			return current
		}
		if fixes == taskID {
			// Cycle detected: return current
			return current
		}
		current = fixes
	}
	return current
}

// UpdateTaskLineage updates the TaskLineage map based on task execution results.
// For failed tasks: resolves to lineage root via resolveLineageRoot, increments ROOT's ConsecutiveFails,
// stores truncated LastError (max 2000 chars), handles chain inheritance via Fixes field.
// For succeeded tasks: resets ConsecutiveFails to 0, clears LastError, updates chain information.
// Only root-keyed entries are maintained in the map — no mirror entries per task.
func UpdateTaskLineage(taskLineage map[string]runstore.TaskLineageEntry, tasks []runstore.Task, failedTaskIDs []string) {
	failedSet := make(map[string]bool)
	for _, id := range failedTaskIDs {
		failedSet[id] = true
	}

	// Track which roots we've already incremented in this call to avoid double-incrementing
	incrementedRoots := make(map[string]bool)

	for i := range tasks {
		task := &tasks[i]

		if failedSet[task.TaskID] {
			// Task failed: resolve to lineage root and update the ROOT's ConsecutiveFails
			var rootTaskID string
			var rootEntry runstore.TaskLineageEntry

			// Determine the root task ID: resolve the task (or its fixed task if present) to the root
			if task.Fixes != "" {
				// Resolve via lineage if Fixes target exists there, otherwise walk the task Fixes chain
				rootTaskID = resolveRootViaTasksFixes(taskLineage, tasks, task.Fixes)
				rootEntry, _ = taskLineage[rootTaskID]
				if _, exists := taskLineage[rootTaskID]; !exists {
					// Root not in lineage yet: initialize it
					rootEntry = runstore.TaskLineageEntry{
						ChainIDs:       []string{rootTaskID},
						OriginalTaskID: rootTaskID,
					}
				}
			} else {
				// Regular task without Fixes: resolve it to its lineage root
				rootTaskID = resolveLineageRoot(taskLineage, task.TaskID)
				var exists bool
				rootEntry, exists = taskLineage[rootTaskID]
				if !exists {
					// Initialize if this is first failure of this root
					rootEntry = runstore.TaskLineageEntry{
						ChainIDs:       []string{rootTaskID},
						OriginalTaskID: rootTaskID,
					}
				}
			}

			// Ensure OriginalTaskID is set (in case entry existed without it)
			if rootEntry.OriginalTaskID == "" {
				rootEntry.OriginalTaskID = rootTaskID
			}

			// Increment ROOT's consecutive failures (only once per root in this call)
			if !incrementedRoots[rootTaskID] {
				rootEntry.ConsecutiveFails++
				incrementedRoots[rootTaskID] = true
			}

			// Track which task ID actually failed last (for planner context)
			rootEntry.LastFailingTaskID = task.TaskID

			// Update ROOT's ChainIDs to include current task if not already there
			if task.TaskID != rootTaskID {
				found := false
				for _, chainID := range rootEntry.ChainIDs {
					if chainID == task.TaskID {
						found = true
						break
					}
				}
				if !found {
					rootEntry.ChainIDs = append(rootEntry.ChainIDs, task.TaskID)
				}
			}

			// Update the ROOT entry in the map (only root-keyed entries are maintained)
			taskLineage[rootTaskID] = rootEntry
		} else if task.Status != "failed" {
			// Task succeeded: reset failure state but keep entry
			entry, exists := taskLineage[task.TaskID]
			if !exists {
				entry = runstore.TaskLineageEntry{
					ChainIDs:       []string{task.TaskID},
					OriginalTaskID: task.TaskID,
				}
			}

			// Reset failure state
			entry.ConsecutiveFails = 0
			entry.LastError = ""

			// If this is a fix task, update the root's ChainIDs to include this task
			// and ensure this entry's ChainIDs[0] points to the root for resolveLineageRoot traversal.
			if task.Fixes != "" {
				fixedTaskID := task.Fixes
				root := resolveRootViaTasksFixes(taskLineage, tasks, fixedTaskID)
				if rootEntry, exists := taskLineage[root]; exists {
					// Append this task's ID to the root's ChainIDs if not already there
					found := false
					for _, chainID := range rootEntry.ChainIDs {
						if chainID == task.TaskID {
							found = true
							break
						}
					}
					if !found {
						rootEntry.ChainIDs = append(rootEntry.ChainIDs, task.TaskID)
						taskLineage[root] = rootEntry
					}
					// Set this entry's ChainIDs[0] to root so resolveLineageRoot can navigate
					// from this task to the root (important for multi-hop fix chains).
					entry.ChainIDs = []string{root, task.TaskID}
					entry.OriginalTaskID = root
				}
			}

			// Ensure ChainIDs is initialized
			if entry.ChainIDs == nil {
				entry.ChainIDs = []string{task.TaskID}
			}

			taskLineage[task.TaskID] = entry

			// Also reset failure counters for chain tasks that are NOT currently failing.
			// Skip tasks in failedSet to avoid undoing increments from this same call.
			for _, chainTaskID := range entry.ChainIDs {
				if chainTaskID != task.TaskID && !failedSet[chainTaskID] {
					chainEntry := taskLineage[chainTaskID]
					chainEntry.ConsecutiveFails = 0
					chainEntry.LastError = ""
					taskLineage[chainTaskID] = chainEntry
				}
			}
		}
		// else: task has 'failed' status and is not in current cycle's failedTaskIDs
		// (i.e., it failed in a prior cycle and wasn't re-executed) — skip it
	}
}

// ShouldEscalateModel returns true if the task is a fix task and the lineage root's
// consecutive failure count meets or exceeds the escalation threshold.
func ShouldEscalateModel(task *runstore.Task, taskLineage map[string]runstore.TaskLineageEntry, modelEscalationThreshold int) bool {
	if task == nil || task.Fixes == "" {
		return false
	}

	fixedTaskID := task.Fixes
	rootTaskID := resolveLineageRoot(taskLineage, fixedTaskID)
	rootEntry, exists := taskLineage[rootTaskID]
	if !exists {
		return false
	}

	return rootEntry.ConsecutiveFails >= modelEscalationThreshold
}

// AppendPriorAttemptErrors appends prior attempt errors to the replan context for lineages
// that have failed >= errorContextThreshold consecutive times.
// Only emits for root entries (where map key == entry.OriginalTaskID) to avoid duplicate error lines.
// Format: "prior-attempt-error: <last-failing-task-id>: <error output>"
// If LastFailingTaskID is empty (backward compat), falls back to the root task ID (map key).
func AppendPriorAttemptErrors(replanContext *[]string, taskLineage map[string]runstore.TaskLineageEntry, errorContextThreshold int) {
	for taskID, entry := range taskLineage {
		// Only include root entries where the map key matches the OriginalTaskID,
		// and ConsecutiveFails >= threshold and has LastError and ChainIDs
		if taskID == entry.OriginalTaskID && entry.ConsecutiveFails >= errorContextThreshold && entry.LastError != "" && len(entry.ChainIDs) > 0 {
			// Use LastFailingTaskID if set; fall back to root task ID for backward compat
			emitTaskID := entry.LastFailingTaskID
			if emitTaskID == "" {
				emitTaskID = taskID
			}
			errorLine := fmt.Sprintf("prior-attempt-error: %s: %s", emitTaskID, entry.LastError)
			*replanContext = append(*replanContext, errorLine)
		}
	}
}
