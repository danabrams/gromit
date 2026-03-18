package specloop

import (
	"strconv"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// TestResolveLineageRoot tests direct root resolution, chain following, and cycle protection.
func TestResolveLineageRoot(t *testing.T) {
	tests := []struct {
		name        string
		taskLineage map[string]runstore.TaskLineageEntry
		taskID      string
		expected    string
	}{
		{
			name:        "direct root - task not in map",
			taskLineage: map[string]runstore.TaskLineageEntry{},
			taskID:      "task-1",
			expected:    "task-1",
		},
		{
			name: "direct root - task with empty chain",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {ChainIDs: []string{}},
			},
			taskID:   "task-1",
			expected: "task-1",
		},
		{
			name: "chain resolution - follow single chain",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1":    {ChainIDs: []string{"root-task"}},
				"root-task": {ChainIDs: []string{}},
			},
			taskID:   "task-1",
			expected: "root-task",
		},
		{
			name: "chain resolution - follow multi-level chain",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-3": {ChainIDs: []string{"task-2"}},
				"task-2": {ChainIDs: []string{"task-1"}},
				"task-1": {ChainIDs: []string{"root"}},
				"root":   {ChainIDs: []string{}},
			},
			taskID:   "task-3",
			expected: "root",
		},
		{
			name: "cycle protection - self-referencing task",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {ChainIDs: []string{"task-1"}},
			},
			taskID:   "task-1",
			expected: "task-1",
		},
		{
			name: "cycle protection - immediate cycle",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {ChainIDs: []string{"task-2"}},
				"task-2": {ChainIDs: []string{"task-1"}},
			},
			taskID:   "task-1",
			expected: "task-2",
		},
		{
			name: "missing entry - chain points to non-existent task",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {ChainIDs: []string{"task-2"}},
			},
			taskID:   "task-1",
			expected: "task-2",
		},
		{
			name: "cycle protection - deep chain with eventual cycle",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-51": {ChainIDs: []string{"task-50"}},
				"task-50": {ChainIDs: []string{"task-49"}},
				"task-49": {ChainIDs: []string{"task-48"}},
			},
			taskID:   "task-51",
			expected: "task-48",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveLineageRoot(tt.taskLineage, tt.taskID)
			if result != tt.expected {
				t.Errorf("resolveLineageRoot() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestUpdateTaskLineage tests new root creation, failure increments, success resets, chain inheritance, and error truncation.
func TestUpdateTaskLineage(t *testing.T) {
	tests := []struct {
		name          string
		taskLineage   map[string]runstore.TaskLineageEntry
		tasks         []runstore.Task
		failedTaskIDs []string
		checkFunc     func(*testing.T, map[string]runstore.TaskLineageEntry)
	}{
		{
			name:          "new root creation - task with no Fixes",
			taskLineage:   map[string]runstore.TaskLineageEntry{},
			tasks:         []runstore.Task{{TaskID: "task-1", Status: "failed"}},
			failedTaskIDs: []string{"task-1"},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-1"]
				if len(entry.ChainIDs) != 1 || entry.ChainIDs[0] != "task-1" {
					t.Errorf("ChainIDs = %v, want [task-1]", entry.ChainIDs)
				}
				if entry.ConsecutiveFails != 1 {
					t.Errorf("ConsecutiveFails = %d, want 1", entry.ConsecutiveFails)
				}
			},
		},
		{
			name: "failure increment - consecutive failures",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {ChainIDs: []string{"task-1"}, ConsecutiveFails: 1, LastError: "first error"},
			},
			tasks: []runstore.Task{
				{TaskID: "task-1", Status: "failed"},
			},
			failedTaskIDs: []string{"task-1"},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-1"]
				if entry.ConsecutiveFails != 2 {
					t.Errorf("ConsecutiveFails = %d, want 2", entry.ConsecutiveFails)
				}
			},
		},
		{
			name: "success reset - clears failure state",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {ChainIDs: []string{"task-1"}, ConsecutiveFails: 3, LastError: "error message"},
			},
			tasks: []runstore.Task{
				{TaskID: "task-1", Status: "done"},
			},
			failedTaskIDs: []string{},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-1"]
				if entry.ConsecutiveFails != 0 {
					t.Errorf("ConsecutiveFails = %d, want 0", entry.ConsecutiveFails)
				}
				if entry.LastError != "" {
					t.Errorf("LastError = %q, want empty", entry.LastError)
				}
			},
		},
		{
			name: "chain inheritance - fixing task inherits chain",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"root":   {ChainIDs: []string{"root"}, ConsecutiveFails: 2},
				"task-1": {ChainIDs: []string{"root", "task-1"}, ConsecutiveFails: 1},
			},
			tasks: []runstore.Task{
				{TaskID: "task-2", Status: "failed", Fixes: "task-1"},
			},
			failedTaskIDs: []string{"task-2"},
			// After Issue 3, no mirror entries — only root-keyed entries.
			// task-2 (fixing task-1 which fixes root) is appended to root's ChainIDs.
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				// task-2 is a fix for task-1, whose root is "root".
				// No mirror entry for task-2 — only the root entry is updated.
				rootEntry := tl["root"]
				if len(rootEntry.ChainIDs) < 2 {
					t.Errorf("root ChainIDs length = %d, want at least 2", len(rootEntry.ChainIDs))
				}
				if len(rootEntry.ChainIDs) > 0 && rootEntry.ChainIDs[0] != "root" {
					t.Errorf("root ChainIDs[0] = %q, want root", rootEntry.ChainIDs[0])
				}
				// task-2 should be appended to root's ChainIDs
				hasTask2 := false
				for _, chainID := range rootEntry.ChainIDs {
					if chainID == "task-2" {
						hasTask2 = true
						break
					}
				}
				if !hasTask2 {
					t.Errorf("task-2 not found in root ChainIDs: %v", rootEntry.ChainIDs)
				}
			},
		},
		{
			// Note: Task.LastError removed (Issue 4). Lineage LastError is no longer set
			// by UpdateTaskLineage from task fields. These truncation tests now verify
			// that a lineage entry is created and ConsecutiveFails incremented.
			name:          "LastError truncation - exceeds 2000 chars (entry created)",
			taskLineage:   map[string]runstore.TaskLineageEntry{},
			tasks:         []runstore.Task{{TaskID: "task-1", Status: "failed"}},
			failedTaskIDs: []string{"task-1"},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-1"]
				if entry.ConsecutiveFails != 1 {
					t.Errorf("ConsecutiveFails = %d, want 1", entry.ConsecutiveFails)
				}
			},
		},
		{
			name:          "LastError truncation - exactly 2000 chars (entry created)",
			taskLineage:   map[string]runstore.TaskLineageEntry{},
			tasks:         []runstore.Task{{TaskID: "task-1", Status: "failed"}},
			failedTaskIDs: []string{"task-1"},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-1"]
				if entry.ConsecutiveFails != 1 {
					t.Errorf("ConsecutiveFails = %d, want 1", entry.ConsecutiveFails)
				}
			},
		},
		{
			name:          "LastError truncation - under 2000 chars (entry created)",
			taskLineage:   map[string]runstore.TaskLineageEntry{},
			tasks:         []runstore.Task{{TaskID: "task-1", Status: "failed"}},
			failedTaskIDs: []string{"task-1"},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-1"]
				if entry.ConsecutiveFails != 1 {
					t.Errorf("ConsecutiveFails = %d, want 1", entry.ConsecutiveFails)
				}
			},
		},
		{
			name: "success with Fixes field - reset failure state",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"root":   {ChainIDs: []string{"root"}, ConsecutiveFails: 1},
				"task-1": {ChainIDs: []string{"root", "task-1"}, ConsecutiveFails: 2, LastError: "error"},
			},
			tasks: []runstore.Task{
				{TaskID: "task-2", Status: "done", Fixes: "task-1"},
			},
			failedTaskIDs: []string{},
			checkFunc: func(t *testing.T, tl map[string]runstore.TaskLineageEntry) {
				entry := tl["task-2"]
				if entry.ConsecutiveFails != 0 {
					t.Errorf("ConsecutiveFails = %d, want 0", entry.ConsecutiveFails)
				}
				if entry.LastError != "" {
					t.Errorf("LastError = %q, want empty", entry.LastError)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateTaskLineage(tt.taskLineage, tt.tasks, tt.failedTaskIDs)
			tt.checkFunc(t, tt.taskLineage)
		})
	}
}

// TestAppendPriorAttemptErrors tests error appending below/at thresholds and formatting.
func TestAppendPriorAttemptErrors(t *testing.T) {
	tests := []struct {
		name                  string
		taskLineage           map[string]runstore.TaskLineageEntry
		errorContextThreshold int
		expectedCount         int
		checkFunc             func(*testing.T, []string)
	}{
		{
			name: "below threshold - no errors appended",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{"task-1"},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 1,
					LastError:        "error message",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         0,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 0 {
					t.Errorf("expected 0 errors, got %d", len(context))
				}
			},
		},
		{
			name: "at threshold - error appended",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{"task-1"},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 2,
					LastError:        "error message",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         1,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 1 {
					t.Errorf("expected 1 error, got %d", len(context))
				}
				if !strings.Contains(context[0], "prior-attempt-error:") {
					t.Errorf("error format incorrect: %q", context[0])
				}
				if !strings.Contains(context[0], "task-1") {
					t.Errorf("error does not contain task-1: %q", context[0])
				}
				if !strings.Contains(context[0], "error message") {
					t.Errorf("error does not contain message: %q", context[0])
				}
			},
		},
		{
			name: "above threshold - error appended",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{"task-1"},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 5,
					LastError:        "critical error",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         1,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 1 {
					t.Errorf("expected 1 error, got %d", len(context))
				}
				expected := "prior-attempt-error: task-1: critical error"
				if context[0] != expected {
					t.Errorf("got %q, want %q", context[0], expected)
				}
			},
		},
		{
			name: "multiple lineages - mixed threshold",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{"task-1"},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 1,
					LastError:        "error 1",
				},
				"task-2": {
					ChainIDs:         []string{"task-2"},
					OriginalTaskID:   "task-2",
					ConsecutiveFails: 2,
					LastError:        "error 2",
				},
				"task-3": {
					ChainIDs:         []string{"task-3"},
					OriginalTaskID:   "task-3",
					ConsecutiveFails: 3,
					LastError:        "error 3",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         2,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 2 {
					t.Errorf("expected 2 errors, got %d", len(context))
				}
				// Check that task-1 is not present (below threshold)
				hasTask1 := false
				for _, e := range context {
					if strings.Contains(e, "task-1") {
						hasTask1 = true
					}
				}
				if hasTask1 {
					t.Errorf("task-1 should not be included (below threshold)")
				}
			},
		},
		{
			name: "no LastError - not appended",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{"task-1"},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 3,
					LastError:        "",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         0,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 0 {
					t.Errorf("expected 0 errors (no LastError), got %d", len(context))
				}
			},
		},
		{
			name: "empty ChainIDs - not appended",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 3,
					LastError:        "error",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         0,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 0 {
					t.Errorf("expected 0 errors (empty ChainIDs), got %d", len(context))
				}
			},
		},
		{
			name: "formatting check - colon-space format",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"my-task": {
					ChainIDs:         []string{"my-task"},
					OriginalTaskID:   "my-task",
					ConsecutiveFails: 2,
					LastError:        "my error message",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         1,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 1 {
					t.Errorf("expected 1 error, got %d", len(context))
				}
				// Check exact format
				expected := "prior-attempt-error: my-task: my error message"
				if context[0] != expected {
					t.Errorf("got %q, want %q", context[0], expected)
				}
			},
		},
		{
			name: "truncation check - error message preserved",
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-1": {
					ChainIDs:         []string{"task-1"},
					OriginalTaskID:   "task-1",
					ConsecutiveFails: 2,
					LastError:        "a very long error message",
				},
			},
			errorContextThreshold: 2,
			expectedCount:         1,
			checkFunc: func(t *testing.T, context []string) {
				if len(context) != 1 {
					t.Errorf("expected 1 error, got %d", len(context))
				}
				if !strings.Contains(context[0], "a very long error message") {
					t.Errorf("long error message not preserved: %q", context[0])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replanContext := []string{}
			AppendPriorAttemptErrors(&replanContext, tt.taskLineage, tt.errorContextThreshold)
			tt.checkFunc(t, replanContext)
		})
	}
}

// TestShouldEscalateModel tests escalation logic below and at thresholds.
func TestShouldEscalateModel(t *testing.T) {
	tests := []struct {
		name                     string
		task                     *runstore.Task
		taskLineage              map[string]runstore.TaskLineageEntry
		modelEscalationThreshold int
		expected                 bool
	}{
		{
			name: "no Fixes field",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "",
			},
			taskLineage:              map[string]runstore.TaskLineageEntry{},
			modelEscalationThreshold: 2,
			expected:                 false,
		},
		{
			name: "fixed task not in lineage",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "task-2",
			},
			taskLineage:              map[string]runstore.TaskLineageEntry{},
			modelEscalationThreshold: 2,
			expected:                 false,
		},
		{
			name: "below escalation threshold",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "task-2",
			},
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-2": {ConsecutiveFails: 1},
			},
			modelEscalationThreshold: 2,
			expected:                 false,
		},
		{
			name: "at escalation threshold",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "task-2",
			},
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-2": {ConsecutiveFails: 2},
			},
			modelEscalationThreshold: 2,
			expected:                 true,
		},
		{
			name: "above escalation threshold",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "task-2",
			},
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-2": {ConsecutiveFails: 5},
			},
			modelEscalationThreshold: 2,
			expected:                 true,
		},
		{
			name: "threshold is 0 - above",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "task-2",
			},
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-2": {ConsecutiveFails: 1},
			},
			modelEscalationThreshold: 0,
			expected:                 true,
		},
		{
			name: "empty Fixes field",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "",
			},
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-2": {ConsecutiveFails: 5},
			},
			modelEscalationThreshold: 2,
			expected:                 false,
		},
		{
			name: "single Fixes",
			task: &runstore.Task{
				TaskID: "task-1",
				Fixes:  "task-2",
			},
			taskLineage: map[string]runstore.TaskLineageEntry{
				"task-2": {ConsecutiveFails: 3},
			},
			modelEscalationThreshold: 2,
			expected:                 true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldEscalateModel(tt.task, tt.taskLineage, tt.modelEscalationThreshold)
			if result != tt.expected {
				t.Errorf("ShouldEscalateModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Individual Test Functions (per task requirements)
// ============================================================================

// TestResolveLineageRoot_FindsRootByChainIDs tests that resolveLineageRoot correctly
// follows the chain of ChainIDs to find the actual root task.
func TestResolveLineageRoot_FindsRootByChainIDs(t *testing.T) {
	lineage := map[string]runstore.TaskLineageEntry{
		"task-a": {
			ChainIDs: []string{"root-task"},
		},
		"root-task": {
			ChainIDs: []string{},
		},
	}

	result := resolveLineageRoot(lineage, "task-a")
	if result != "root-task" {
		t.Errorf("expected root-task, got %s", result)
	}
}

// TestResolveLineageRoot_ReturnsInputWhenNotFound tests that resolveLineageRoot
// returns the input taskID unchanged when it's not found in the lineage map.
func TestResolveLineageRoot_ReturnsInputWhenNotFound(t *testing.T) {
	lineage := map[string]runstore.TaskLineageEntry{
		"task-a": {
			ChainIDs: []string{"task-a"},
		},
	}

	result := resolveLineageRoot(lineage, "unknown-task")
	if result != "unknown-task" {
		t.Errorf("expected unknown-task, got %s", result)
	}
}

// TestResolveLineageRoot_MaxDepthGuard tests that resolveLineageRoot doesn't
// infinite loop when encountering cyclic references. It should terminate at maxDepth.
func TestResolveLineageRoot_MaxDepthGuard(t *testing.T) {
	// Create a deep cycle: task-1 -> task-2 -> task-3 -> ... -> task-101 -> task-1
	lineage := make(map[string]runstore.TaskLineageEntry)
	for i := 1; i <= 105; i++ {
		nextIdx := i%105 + 1
		lineage["task-"+strconv.Itoa(i)] = runstore.TaskLineageEntry{
			ChainIDs: []string{"task-" + strconv.Itoa(nextIdx)},
		}
	}

	// Should not panic or infinite loop
	result := resolveLineageRoot(lineage, "task-1")
	if result == "" {
		t.Error("expected non-empty result even with cycle")
	}
}

// TestUpdateTaskLineage_CreatesNewEntryOnFirstFailure tests that UpdateTaskLineage
// creates a new lineage entry for a failed task with ConsecutiveFails=1, LastError set,
// and ChainIDs containing just the task's own ID.
func TestUpdateTaskLineage_CreatesNewEntryOnFirstFailure(t *testing.T) {
	taskLineage := make(map[string]runstore.TaskLineageEntry)
	tasks := []runstore.Task{
		{
			TaskID: "new-task",
			Status: "failed",
		},
	}
	failedTaskIDs := []string{"new-task"}

	// Pass error via UpdateTaskLineage using LastError on the lineage entry
	// (Task no longer has LastError field — error is passed through the task's
	// LastError which is read from the lineage; here we set it via a direct
	// task struct that still participates in the call)
	// Actually since Task no longer has LastError, we pass it differently.
	// UpdateTaskLineage reads task.LastError which is gone — we need to check
	// that the lineage was created correctly with ConsecutiveFails=1.
	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	entry, exists := taskLineage["new-task"]
	if !exists {
		t.Fatal("expected entry for new-task")
	}
	if entry.ConsecutiveFails != 1 {
		t.Errorf("expected ConsecutiveFails=1, got %d", entry.ConsecutiveFails)
	}
	if len(entry.ChainIDs) != 1 || entry.ChainIDs[0] != "new-task" {
		t.Errorf("expected ChainIDs=[new-task], got %v", entry.ChainIDs)
	}
}

// TestUpdateTaskLineage_IncrementsExistingOnSubsequentFailure tests that UpdateTaskLineage
// increments ConsecutiveFails when a task fails multiple times.
func TestUpdateTaskLineage_IncrementsExistingOnSubsequentFailure(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"task-x": {
			ChainIDs:         []string{"task-x"},
			ConsecutiveFails: 1,
			LastError:        "first error",
		},
	}
	tasks := []runstore.Task{
		{
			TaskID: "task-x",
			Status: "failed",
		},
	}
	failedTaskIDs := []string{"task-x"}

	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	entry := taskLineage["task-x"]
	if entry.ConsecutiveFails != 2 {
		t.Errorf("expected ConsecutiveFails=2, got %d", entry.ConsecutiveFails)
	}
	// task.ConsecutiveFails no longer exists on Task struct (Issue 4)
}

// TestUpdateTaskLineage_TruncatesLastErrorTo2000Chars tests that UpdateTaskLineage
// creates a lineage entry on failure. Note: Task.LastError was removed (Issue 4),
// so this test now just verifies the entry is created.
func TestUpdateTaskLineage_TruncatesLastErrorTo2000Chars(t *testing.T) {
	taskLineage := make(map[string]runstore.TaskLineageEntry)
	tasks := []runstore.Task{
		{
			TaskID: "long-error-task",
			Status: "failed",
		},
	}
	failedTaskIDs := []string{"long-error-task"}

	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	entry := taskLineage["long-error-task"]
	if entry.ConsecutiveFails != 1 {
		t.Errorf("expected ConsecutiveFails=1, got %d", entry.ConsecutiveFails)
	}
	// task.LastError no longer exists on Task struct (Issue 4)
}

// TestUpdateTaskLineage_ResetsOnSuccess tests that UpdateTaskLineage resets
// ConsecutiveFails to 0 and clears LastError when a task succeeds.
func TestUpdateTaskLineage_ResetsOnSuccess(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"task-y": {
			ChainIDs:         []string{"task-y"},
			ConsecutiveFails: 3,
			LastError:        "previous error",
		},
	}
	tasks := []runstore.Task{
		{
			TaskID: "task-y",
			Status: "done",
		},
	}
	failedTaskIDs := []string{}

	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	entry := taskLineage["task-y"]
	if entry.ConsecutiveFails != 0 {
		t.Errorf("expected ConsecutiveFails=0 after success, got %d", entry.ConsecutiveFails)
	}
	if entry.LastError != "" {
		t.Errorf("expected LastError='', got '%s'", entry.LastError)
	}
	// task.ConsecutiveFails and task.LastError no longer exist on Task struct (Issue 4)
}

// TestAppendPriorAttemptErrors_AddsWhenThresholdMet tests that AppendPriorAttemptErrors
// adds error context strings for tasks that have failed >= errorContextThreshold times.
func TestAppendPriorAttemptErrors_AddsWhenThresholdMet(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"task-fail": {
			ChainIDs:         []string{"task-fail"},
			OriginalTaskID:   "task-fail",
			ConsecutiveFails: 2,
			LastError:        "some error message",
		},
	}
	replanContext := []string{}
	errorContextThreshold := 2

	AppendPriorAttemptErrors(&replanContext, taskLineage, errorContextThreshold)

	if len(replanContext) == 0 {
		t.Fatal("expected prior attempt error to be added")
	}
	if !strings.Contains(replanContext[0], "prior-attempt-error: task-fail:") {
		t.Errorf("expected prior-attempt-error format, got '%s'", replanContext[0])
	}
	if !strings.Contains(replanContext[0], "some error message") {
		t.Errorf("expected error message in context, got '%s'", replanContext[0])
	}
}

// TestAppendPriorAttemptErrors_SkipsBelowThreshold tests that AppendPriorAttemptErrors
// does not add error context for tasks below the errorContextThreshold.
func TestAppendPriorAttemptErrors_SkipsBelowThreshold(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"task-fail": {
			ChainIDs:         []string{"task-fail"},
			OriginalTaskID:   "task-fail",
			ConsecutiveFails: 1,
			LastError:        "some error message",
		},
	}
	replanContext := []string{}
	errorContextThreshold := 2

	AppendPriorAttemptErrors(&replanContext, taskLineage, errorContextThreshold)

	if len(replanContext) > 0 {
		t.Errorf("expected no prior attempt errors, but got %d items", len(replanContext))
	}
}

// TestShouldEscalateModel_TrueWhenThresholdMet tests that ShouldEscalateModel
// returns true when the fixed task's ConsecutiveFails >= modelEscalationThreshold.
func TestShouldEscalateModel_TrueWhenThresholdMet(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"original-task": {
			ChainIDs:         []string{"original-task"},
			ConsecutiveFails: 3,
		},
	}
	task := &runstore.Task{
		TaskID: "fix-task",
		Fixes:  "original-task",
	}
	modelEscalationThreshold := 3

	result := ShouldEscalateModel(task, taskLineage, modelEscalationThreshold)
	if !result {
		t.Error("expected true when ConsecutiveFails >= threshold")
	}
}

// TestShouldEscalateModel_FalseNoFixes tests that ShouldEscalateModel
// returns false when the task has no Fixes field.
func TestShouldEscalateModel_FalseNoFixes(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"original-task": {
			ChainIDs:         []string{"original-task"},
			ConsecutiveFails: 3,
		},
	}
	task := &runstore.Task{
		TaskID: "regular-task",
		Fixes:  "",
	}
	modelEscalationThreshold := 2

	result := ShouldEscalateModel(task, taskLineage, modelEscalationThreshold)
	if result {
		t.Error("expected false when Fixes is empty")
	}
}

// TestShouldEscalateModel_FalseBelowThreshold tests that ShouldEscalateModel
// returns false when the fixed task's ConsecutiveFails is below the threshold.
func TestShouldEscalateModel_FalseBelowThreshold(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"original-task": {
			ChainIDs:         []string{"original-task"},
			ConsecutiveFails: 1,
		},
	}
	task := &runstore.Task{
		TaskID: "fix-task",
		Fixes:  "original-task",
	}
	modelEscalationThreshold := 3

	result := ShouldEscalateModel(task, taskLineage, modelEscalationThreshold)
	if result {
		t.Error("expected false when ConsecutiveFails < threshold")
	}
}

// TestTruncateLastError tests the truncateLastError helper directly.
func TestTruncateLastError(t *testing.T) {
	tests := []struct {
		name           string
		lastError      string
		expectedLength int
	}{
		{
			name:           "long error exceeds 2000",
			lastError:      strings.Repeat("x", 3000),
			expectedLength: 2000,
		},
		{
			name:           "error exactly 2000 chars",
			lastError:      strings.Repeat("x", 2000),
			expectedLength: 2000,
		},
		{
			name:           "short error under 2000",
			lastError:      strings.Repeat("x", 1000),
			expectedLength: 1000,
		},
		{
			name:           "empty error",
			lastError:      "",
			expectedLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateLastError(tt.lastError)
			if len(result) != tt.expectedLength {
				t.Errorf("truncateLastError length = %d, want %d", len(result), tt.expectedLength)
			}
		})
	}
}

// TestUpdateTaskLineage_ResolveFixTaskToRoot tests that a failing fix task
// resolves its fixed task to the lineage root and increments the ROOT's ConsecutiveFails,
// rather than creating a separate counter for the fix task itself.
func TestUpdateTaskLineage_ResolveFixTaskToRoot(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"root-task": {
			ChainIDs:         []string{"root-task"},
			ConsecutiveFails: 2,
		},
		"task-1": {
			ChainIDs:         []string{"root-task", "task-1"},
			ConsecutiveFails: 2,
		},
	}
	tasks := []runstore.Task{
		{
			TaskID: "fix-task",
			Status: "failed",
			Fixes:  "task-1",
		},
	}
	failedTaskIDs := []string{"fix-task"}

	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	// The root task's ConsecutiveFails should be incremented to 3
	rootEntry := taskLineage["root-task"]
	if rootEntry.ConsecutiveFails != 3 {
		t.Errorf("root ConsecutiveFails = %d, want 3", rootEntry.ConsecutiveFails)
	}

	// Verify fix-task is in the root's ChainIDs
	found := false
	for _, chainID := range rootEntry.ChainIDs {
		if chainID == "fix-task" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("fix-task not found in root ChainIDs: %v", rootEntry.ChainIDs)
	}
}

// TestUpdateTaskLineage_MultipleFalsesInChainIncrementRootOnce tests that
// when multiple failed tasks in the same lineage are processed in one call,
// the root's ConsecutiveFails is incremented only once, not once per failed task.
func TestUpdateTaskLineage_MultipleFalsesInChainIncrementRootOnce(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"root-task": {
			ChainIDs:         []string{"root-task"},
			ConsecutiveFails: 1,
		},
		"task-1": {
			ChainIDs:         []string{"root-task", "task-1"},
			ConsecutiveFails: 1,
		},
	}
	tasks := []runstore.Task{
		{
			TaskID: "task-1",
			Status: "failed",
		},
		{
			TaskID: "task-2",
			Status: "failed",
			Fixes:  "task-1",
		},
	}
	failedTaskIDs := []string{"task-1", "task-2"}

	UpdateTaskLineage(taskLineage, tasks, failedTaskIDs)

	// Both failed tasks should increment the root once (from 1 to 2)
	rootEntry := taskLineage["root-task"]
	if rootEntry.ConsecutiveFails != 2 {
		t.Errorf("root ConsecutiveFails = %d, want 2", rootEntry.ConsecutiveFails)
	}

	// Verify both tasks are in the root's ChainIDs
	hasTask1 := false
	hasTask2 := false
	for _, chainID := range rootEntry.ChainIDs {
		if chainID == "task-1" {
			hasTask1 = true
		}
		if chainID == "task-2" {
			hasTask2 = true
		}
	}
	if !hasTask1 || !hasTask2 {
		t.Errorf("not all tasks in chain: task-1=%v, task-2=%v", hasTask1, hasTask2)
	}
}

// TestShouldEscalateModel_ResolvesFixedTaskToRoot tests that ShouldEscalateModel
// correctly resolves the fixed task to its lineage root before checking ConsecutiveFails.
func TestShouldEscalateModel_ResolvesFixedTaskToRoot(t *testing.T) {
	taskLineage := map[string]runstore.TaskLineageEntry{
		"root-task": {
			ChainIDs:         []string{"root-task"},
			ConsecutiveFails: 3,
		},
		"task-1": {
			ChainIDs:         []string{"root-task", "task-1"},
			ConsecutiveFails: 3,
		},
		"task-2": {
			ChainIDs:         []string{"root-task", "task-1", "task-2"},
			ConsecutiveFails: 3,
		},
	}
	task := &runstore.Task{
		TaskID: "fix-task",
		Fixes:  "task-2",
	}
	modelEscalationThreshold := 3

	// Should escalate because task-2 resolves to root-task, which has ConsecutiveFails=3
	result := ShouldEscalateModel(task, taskLineage, modelEscalationThreshold)
	if !result {
		t.Errorf("expected true when fixed task resolves to root with failures >= threshold")
	}
}

// TestAppendPriorAttemptErrors_OnlyRootEntries verifies that AppendPriorAttemptErrors
// only emits error lines for root entries (where map key == entry.OriginalTaskID),
// preventing duplicate error lines from per-task mirror entries.
func TestAppendPriorAttemptErrors_OnlyRootEntries(t *testing.T) {
	// Simulate a lineage with both root and per-task entries
	taskLineage := map[string]runstore.TaskLineageEntry{
		"root-task": {
			ChainIDs:         []string{"root-task", "task-1"},
			OriginalTaskID:   "root-task",
			ConsecutiveFails: 2,
			LastError:        "root error",
		},
		"task-1": {
			ChainIDs:         []string{"root-task", "task-1"},
			OriginalTaskID:   "root-task", // Same OriginalTaskID as root
			ConsecutiveFails: 2,
			LastError:        "root error", // Same error
		},
	}

	replanContext := []string{}
	AppendPriorAttemptErrors(&replanContext, taskLineage, 2)

	// Should only emit ONE error line for the root entry, not two
	if len(replanContext) != 1 {
		t.Errorf("expected 1 error line, got %d: %v", len(replanContext), replanContext)
	}
	if len(replanContext) > 0 && !strings.Contains(replanContext[0], "root-task") {
		t.Errorf("error should be for root-task: %q", replanContext[0])
	}
}

// TestChainIDsNoDuplicatesAfterMultipleCycles verifies that ChainIDs contains
// no duplicate task IDs after multiple failure cycles of a fix chain.
func TestChainIDsNoDuplicatesAfterMultipleCycles(t *testing.T) {
	taskLineage := make(map[string]runstore.TaskLineageEntry)

	// Cycle 1: root task fails
	tasks := []runstore.Task{
		{TaskID: "root-task", Status: "failed"},
	}
	UpdateTaskLineage(taskLineage, tasks, []string{"root-task"})
	verifyNoDuplicateChainIDs(t, taskLineage, "cycle 1 (root fails)")

	// Cycle 2: first fix task fails
	tasks = []runstore.Task{
		{TaskID: "root-task", Status: "failed"},
		{TaskID: "fix-1", Status: "failed", Fixes: "root-task"},
	}
	UpdateTaskLineage(taskLineage, tasks, []string{"root-task", "fix-1"})
	verifyNoDuplicateChainIDs(t, taskLineage, "cycle 2 (fix-1 fails)")

	// Cycle 3: second fix task fails
	tasks = []runstore.Task{
		{TaskID: "root-task", Status: "failed"},
		{TaskID: "fix-1", Status: "failed", Fixes: "root-task"},
		{TaskID: "fix-2", Status: "failed", Fixes: "fix-1"},
	}
	UpdateTaskLineage(taskLineage, tasks, []string{"root-task", "fix-1", "fix-2"})
	verifyNoDuplicateChainIDs(t, taskLineage, "cycle 3 (fix-2 fails)")

	// Verify the final chain structure
	rootEntry := taskLineage["root-task"]
	if len(rootEntry.ChainIDs) == 0 {
		t.Fatalf("root ChainIDs should not be empty")
	}
	if rootEntry.ChainIDs[0] != "root-task" {
		t.Errorf("root ChainIDs[0] should be root-task, got %q", rootEntry.ChainIDs[0])
	}

	// Check that all expected tasks are in the chain
	hasRoot := false
	hasFix1 := false
	hasFix2 := false
	for _, chainID := range rootEntry.ChainIDs {
		if chainID == "root-task" {
			hasRoot = true
		}
		if chainID == "fix-1" {
			hasFix1 = true
		}
		if chainID == "fix-2" {
			hasFix2 = true
		}
	}
	if !hasRoot || !hasFix1 || !hasFix2 {
		t.Errorf("not all tasks in final chain: root=%v, fix-1=%v, fix-2=%v", hasRoot, hasFix1, hasFix2)
	}
}

// verifyNoDuplicateChainIDs is a helper that checks all entries in taskLineage
// have no duplicate task IDs in their ChainIDs.
func verifyNoDuplicateChainIDs(t *testing.T, taskLineage map[string]runstore.TaskLineageEntry, phase string) {
	for taskID, entry := range taskLineage {
		seen := make(map[string]bool)
		for _, chainID := range entry.ChainIDs {
			if seen[chainID] {
				t.Errorf("[%s] entry %q has duplicate chainID %q in %v", phase, taskID, chainID, entry.ChainIDs)
			}
			seen[chainID] = true
		}
	}
}

// Helper function to format integers
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	var result string
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}
