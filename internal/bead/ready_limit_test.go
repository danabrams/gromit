// This file contains acceptance tests for the Ready batch limit optimization.
// These tests verify that Ready() uses --limit 3 instead of --limit 10 to reduce
// subprocess overhead while still providing sufficient margin for epic filtering.

package bead

import (
	"strings"
	"testing"
)

// TestReadyUsesLimit3 verifies that Ready() calls bd ready with --limit 3 flag.
// Expected failure: Client.runFn field does not exist yet (compilation will fail).
// After implementation, this field will allow injecting a mock run function to verify
// command arguments without spawning subprocesses. The test verifies Ready() calls
// run() with: bd ready --json --limit 3 (not --limit 10).
func TestReadyUsesLimit3(t *testing.T) {
	tests := []struct {
		name     string
		bdOutput string
		wantArgs []string
	}{
		{
			name: "ready with task bead",
			bdOutput: `[{
				"id": "task-001",
				"title": "Test task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantArgs: []string{"ready", "--json", "--limit", "3"},
		},
		{
			name: "ready with epic then task - filters epic",
			bdOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantArgs: []string{"ready", "--json", "--limit", "3"},
		},
		{
			name:     "ready with no beads",
			bdOutput: `[]`,
			wantArgs: []string{"ready", "--json", "--limit", "3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			mockRun := func(args ...string) (string, error) {
				capturedArgs = args
				return tt.bdOutput, nil
			}

			c := &Client{
				binary: "bd",
				runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
			}

			_, err := c.Ready()
			if err != nil {
				t.Fatalf("Ready() unexpected error: %v", err)
			}

			// Verify the command arguments match expected (especially --limit 3)
			if len(capturedArgs) != len(tt.wantArgs) {
				t.Errorf("Ready() called run() with %d args, want %d\nGot:  %v\nWant: %v",
					len(capturedArgs), len(tt.wantArgs), capturedArgs, tt.wantArgs)
				return
			}

			for i := range capturedArgs {
				if capturedArgs[i] != tt.wantArgs[i] {
					t.Errorf("Ready() arg[%d] = %q, want %q\nGot:  %v\nWant: %v",
						i, capturedArgs[i], tt.wantArgs[i], capturedArgs, tt.wantArgs)
				}
			}
		})
	}
}

// TestReadyLimit3StillFiltersEpicsCorrectly verifies that the smaller batch size
// of 3 (down from 10) still provides sufficient margin for epic filtering.
// Expected failure: Client.runFn field does not exist yet (compilation will fail).
// This test verifies parseBeadOutputExcluding logic works with limit 3.
func TestReadyLimit3StillFiltersEpicsCorrectly(t *testing.T) {
	tests := []struct {
		name     string
		bdOutput string
		wantID   string
		wantNil  bool
	}{
		{
			name: "single epic in batch - returns nil",
			bdOutput: `[{
				"id": "epic-001",
				"title": "Epic",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantNil: true,
		},
		{
			name: "two epics then task - returns task",
			bdOutput: `[{
				"id": "epic-001",
				"title": "Epic 1",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-002",
				"title": "Epic 2",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:  "task-001",
			wantNil: false,
		},
		{
			name: "three epics only - returns nil",
			bdOutput: `[{
				"id": "epic-001",
				"title": "Epic 1",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-002",
				"title": "Epic 2",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-003",
				"title": "Epic 3",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}]`,
			wantNil: true,
		},
		{
			name: "task first - returns immediately",
			bdOutput: `[{
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			wantID:  "task-001",
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRun := func(args ...string) (string, error) {
				// Verify --limit 3 is being used
				limitIdx := -1
				for i, arg := range args {
					if arg == "--limit" && i+1 < len(args) {
						limitIdx = i + 1
						break
					}
				}
				if limitIdx == -1 {
					t.Error("Ready() should call run() with --limit flag")
				} else if args[limitIdx] != "3" {
					t.Errorf("Ready() should use --limit 3, got --limit %s", args[limitIdx])
				}
				return tt.bdOutput, nil
			}

			c := &Client{
				binary: "bd",
				runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
			}

			got, err := c.Ready()
			if err != nil {
				t.Fatalf("Ready() unexpected error: %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("Ready() expected nil bead, got %+v", got)
				}
			} else {
				if got == nil {
					t.Fatal("Ready() expected non-nil bead, got nil")
				}
				if got.ID != tt.wantID {
					t.Errorf("Ready() returned bead ID = %q, want %q", got.ID, tt.wantID)
				}
			}
		})
	}
}

// TestReadyAnyStillUsesLimit1 verifies that ReadyAny() continues to use --limit 1
// and is not affected by the Ready() limit change.
// Expected failure: Client.runFn field does not exist yet (compilation will fail).
// This test ensures ReadyAny behavior remains unchanged.
func TestReadyAnyStillUsesLimit1(t *testing.T) {
	bdOutput := `[{
		"id": "epic-001",
		"title": "Epic",
		"priority": 0,
		"issue_type": "epic",
		"status": "open"
	}]`

	var capturedArgs []string
	mockRun := func(args ...string) (string, error) {
		capturedArgs = args
		return bdOutput, nil
	}

	c := &Client{
		binary: "bd",
		runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
	}

	_, err := c.ReadyAny()
	if err != nil {
		t.Fatalf("ReadyAny() unexpected error: %v", err)
	}

	wantArgs := []string{"ready", "--json", "--limit", "1"}
	if len(capturedArgs) != len(wantArgs) {
		t.Errorf("ReadyAny() called run() with %d args, want %d\nGot:  %v\nWant: %v",
			len(capturedArgs), len(wantArgs), capturedArgs, wantArgs)
		return
	}

	for i := range capturedArgs {
		if capturedArgs[i] != wantArgs[i] {
			t.Errorf("ReadyAny() arg[%d] = %q, want %q\nGot:  %v\nWant: %v",
				i, capturedArgs[i], wantArgs[i], capturedArgs, wantArgs)
		}
	}
}

// TestReadyLimit3PerformanceCharacteristics documents that the change from --limit 10
// to --limit 3 maintains correctness while reducing serialization/parsing overhead.
// Expected failure: The current implementation uses --limit 10, not --limit 3.
// This test will fail when run against the current code because it will see "10" not "3".
func TestReadyLimit3PerformanceCharacteristics(t *testing.T) {
	tests := []struct {
		name           string
		bdOutput       string
		description    string
		epicCount      int
		nonEpicCount   int
		expectsNonNull bool
	}{
		{
			name: "worst case: 2 consecutive epics at top",
			bdOutput: `[{
				"id": "epic-001",
				"title": "Epic 1",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-002",
				"title": "Epic 2",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "task-001",
				"title": "Task",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			description:    "Limit 3 is sufficient for 2 consecutive epics + 1 task",
			epicCount:      2,
			nonEpicCount:   1,
			expectsNonNull: true,
		},
		{
			name: "common case: no epics at top",
			bdOutput: `[{
				"id": "task-001",
				"title": "Task 1",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "task-002",
				"title": "Task 2",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}, {
				"id": "task-003",
				"title": "Task 3",
				"priority": 1,
				"issue_type": "task",
				"status": "open"
			}]`,
			description:    "Limit 3 handles common case of tasks at top efficiently",
			epicCount:      0,
			nonEpicCount:   3,
			expectsNonNull: true,
		},
		{
			name: "edge case: all 3 slots are epics",
			bdOutput: `[{
				"id": "epic-001",
				"title": "Epic 1",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-002",
				"title": "Epic 2",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}, {
				"id": "epic-003",
				"title": "Epic 3",
				"priority": 0,
				"issue_type": "epic",
				"status": "open"
			}]`,
			description:    "Limit 3 correctly returns nil when all are epics",
			epicCount:      3,
			nonEpicCount:   0,
			expectsNonNull: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedArgs []string
			mockRun := func(args ...string) (string, error) {
				capturedArgs = args
				return tt.bdOutput, nil
			}

			c := &Client{
				binary: "bd",
				runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
			}

			got, err := c.Ready()
			if err != nil {
				t.Fatalf("Ready() unexpected error: %v", err)
			}

			// First verify the limit is 3, not 10
			var limitValue string
			for i, arg := range capturedArgs {
				if arg == "--limit" && i+1 < len(capturedArgs) {
					limitValue = capturedArgs[i+1]
					break
				}
			}

			if limitValue != "3" {
				t.Errorf("Ready() should use --limit 3, got --limit %s\nDescription: %s",
					limitValue, tt.description)
			}

			// Verify behavior is correct with limit 3
			if tt.expectsNonNull {
				if got == nil {
					t.Errorf("Ready() expected non-nil bead for case: %s", tt.description)
				}
			} else {
				if got != nil {
					t.Errorf("Ready() expected nil for case: %s, got bead %+v", tt.description, got)
				}
			}
		})
	}
}

// TestReadyBatchSizeReductionIntent documents the reasoning for the --limit 3 change.
// This is not a behavioral test but documents the optimization rationale.
// Expected failure: Current code uses --limit 10, violating the intended optimization.
func TestReadyBatchSizeReductionIntent(t *testing.T) {
	// This test captures the intent: reduce from 10 to 3 to minimize overhead
	// while still providing comfortable margin for epic filtering.

	mockRun := func(args ...string) (string, error) {
		// Verify limit is 3, not the old value of 10
		for i, arg := range args {
			if arg == "--limit" && i+1 < len(args) {
				if args[i+1] == "10" {
					t.Errorf("Ready() still uses old batch size of 10. Expected optimization to --limit 3 has not been applied.")
				} else if args[i+1] != "3" {
					t.Errorf("Ready() uses unexpected limit value %q, expected 3", args[i+1])
				}
			}
		}
		// Return minimal valid output
		return `[{"id":"task-001","title":"Task","priority":1,"issue_type":"task","status":"open"}]`, nil
	}

	c := &Client{
		binary: "bd",
		runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
	}

	_, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() unexpected error: %v", err)
	}

	// The mockRun function above contains the assertions.
	// If we reach here without mockRun reporting errors via t.Errorf,
	// then the limit is correctly set to 3.
}

// TestReadyLimitValueIsNumeric verifies that the limit argument is a valid number.
// Expected failure: Client.runFn field does not exist yet (compilation will fail).
// This test ensures the implementation doesn't accidentally use non-numeric limit.
func TestReadyLimitValueIsNumeric(t *testing.T) {
	mockRun := func(args ...string) (string, error) {
		// Find --limit flag and verify next arg is numeric
		for i, arg := range args {
			if arg == "--limit" && i+1 < len(args) {
				limitVal := args[i+1]
				// Check it's numeric and equals "3"
				if limitVal != "3" {
					t.Errorf("Ready() limit value = %q, want \"3\"", limitVal)
				}
				// Verify it's a valid number by checking all characters are digits
				for _, ch := range limitVal {
					if ch < '0' || ch > '9' {
						t.Errorf("Ready() limit value %q contains non-digit character", limitVal)
					}
				}
			}
		}
		return `[]`, nil
	}

	c := &Client{
		binary: "bd",
		runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
	}

	_, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() unexpected error: %v", err)
	}
}

// TestReadyCommandStructureWithLimit3 verifies the complete command structure
// when using --limit 3, ensuring all flags are in correct order.
// Expected failure: Client.runFn field does not exist yet (compilation will fail).
// This test verifies the full bd ready command with limit 3.
func TestReadyCommandStructureWithLimit3(t *testing.T) {
	var capturedArgs []string
	mockRun := func(args ...string) (string, error) {
		capturedArgs = args
		return `[{"id":"t1","title":"T","priority":1,"issue_type":"task","status":"open"}]`, nil
	}

	c := &Client{
		binary: "bd",
		runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
	}

	_, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}

	// Verify exact command structure: bd ready --json --limit 3
	expected := []string{"ready", "--json", "--limit", "3"}
	if len(capturedArgs) != len(expected) {
		t.Errorf("Ready() command has %d args, want %d\nGot:  %v\nWant: %v",
			len(capturedArgs), len(expected), capturedArgs, expected)
		return
	}

	for i, want := range expected {
		if capturedArgs[i] != want {
			t.Errorf("Ready() command arg[%d] = %q, want %q", i, capturedArgs[i], want)
		}
	}

	// Additionally verify that there's no trace of the old limit value (10)
	for _, arg := range capturedArgs {
		if arg == "10" {
			t.Errorf("Ready() command still contains old limit value '10', should be '3'")
		}
	}
}

// TestReadyDoesNotUseLimit10Anymore verifies the old limit value is no longer used.
// Expected failure: Current implementation uses "--limit", "10" which this test rejects.
// This test will fail until the code is changed from 10 to 3.
func TestReadyDoesNotUseLimit10Anymore(t *testing.T) {
	mockRun := func(args ...string) (string, error) {
		// Scan for the old limit value
		argStr := strings.Join(args, " ")
		if strings.Contains(argStr, "10") {
			t.Errorf("Ready() command contains '10' which is the old limit value. Command: %s", argStr)
		}

		// Verify it uses 3 instead
		foundLimit3 := false
		for i, arg := range args {
			if arg == "--limit" && i+1 < len(args) && args[i+1] == "3" {
				foundLimit3 = true
				break
			}
		}
		if !foundLimit3 {
			t.Errorf("Ready() command does not use --limit 3. Command: %s", argStr)
		}

		return `[]`, nil
	}

	c := &Client{
		binary: "bd",
		runFn:  mockRun, // Expected failure: runFn field does not exist on Client struct
	}

	_, err := c.Ready()
	if err != nil {
		t.Fatalf("Ready() error = %v", err)
	}
}
