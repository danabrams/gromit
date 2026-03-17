package specloop

import (
	"slices"
	"testing"
)

// TestFailureHistory is the main test suite for failure history functionality
func TestFailureHistory(t *testing.T) {
	t.Run("TestExtractTestFailureKeys", testExtractTestFailureKeys)
	t.Run("TestExtractContractFailureKeys", testExtractContractFailureKeys)
	t.Run("TestUpdateFailureHistory", testUpdateFailureHistory)
	t.Run("TestAnnotateWithPersistentFailureHints", testAnnotateWithPersistentFailureHints)
}

func testExtractTestFailureKeys(t *testing.T) {
	tests := []struct {
		name     string
		failures []string
		want     []string
	}{
		{
			name: "extract single test failure",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
			want: []string{"TestFoo"},
		},
		{
			name: "extract multiple test failures",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
				"--- FAIL: TestBar (0.02s)",
				"--- FAIL: TestBaz (0.03s)",
			},
			want: []string{"TestFoo", "TestBar", "TestBaz"},
		},
		{
			name: "ignore non-matching lines",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
				"some other error",
				"contract:something — failed",
				"--- PASS: TestBar (0.02s)",
			},
			want: []string{"TestFoo"},
		},
		{
			name:     "empty input returns empty list",
			failures: []string{},
			want:     []string{},
		},
		{
			name: "no matching failures returns empty list",
			failures: []string{
				"some random error",
				"another error",
			},
			want: []string{},
		},
		{
			name: "test name with multiple words uses first word",
			failures: []string{
				"--- FAIL: TestFoo and other text (0.01s)",
			},
			want: []string{"TestFoo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTestFailureKeys(tt.failures)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ExtractTestFailureKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testExtractContractFailureKeys(t *testing.T) {
	tests := []struct {
		name     string
		failures []string
		want     []string
	}{
		{
			name: "extract single contract failure",
			failures: []string{
				"contract:add-works — file_contains failed: expected output not found",
			},
			want: []string{"contract:add-works"},
		},
		{
			name: "extract multiple contract failures",
			failures: []string{
				"contract:add-works — file_contains failed: expected output not found",
				"contract:list-shows-items — output_matches failed: regex mismatch",
				"contract:delete-removes — file_missing failed: file still exists",
			},
			want: []string{"contract:add-works", "contract:list-shows-items", "contract:delete-removes"},
		},
		{
			name: "ignore non-contract failures",
			failures: []string{
				"contract:add-works — file_contains failed: expected output not found",
				"--- FAIL: TestFoo (0.01s)",
				"some other error",
				"contract:list-shows-items — output_matches failed: regex mismatch",
			},
			want: []string{"contract:add-works", "contract:list-shows-items"},
		},
		{
			name:     "empty input returns empty list",
			failures: []string{},
			want:     []string{},
		},
		{
			name: "no matching failures returns empty list",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
				"some error",
			},
			want: []string{},
		},
		{
			name: "handles extra whitespace around delimiter",
			failures: []string{
				"contract:test-case  —  some details here",
			},
			want: []string{"contract:test-case"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractContractFailureKeys(tt.failures)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ExtractContractFailureKeys() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testUpdateFailureHistory(t *testing.T) {
	tests := []struct {
		name        string
		history     map[string]int
		currentKeys []string
		want        map[string]int
	}{
		{
			name:        "increment present keys",
			history:     map[string]int{"TestFoo": 1, "TestBar": 2},
			currentKeys: []string{"TestFoo", "TestBar"},
			want:        map[string]int{"TestFoo": 2, "TestBar": 3},
		},
		{
			name:        "remove absent keys",
			history:     map[string]int{"TestFoo": 1, "TestBar": 2},
			currentKeys: []string{"TestFoo"},
			want:        map[string]int{"TestFoo": 2},
		},
		{
			name:        "add new keys",
			history:     map[string]int{"TestFoo": 1},
			currentKeys: []string{"TestFoo", "TestBar"},
			want:        map[string]int{"TestFoo": 2, "TestBar": 1},
		},
		{
			name:        "handle empty history",
			history:     map[string]int{},
			currentKeys: []string{"TestFoo", "TestBar"},
			want:        map[string]int{"TestFoo": 1, "TestBar": 1},
		},
		{
			name:        "clear all keys when no current keys",
			history:     map[string]int{"TestFoo": 1, "TestBar": 2},
			currentKeys: []string{},
			want:        map[string]int{},
		},
		{
			name:        "mixed additions and removals",
			history:     map[string]int{"TestFoo": 1, "TestBar": 2, "TestBaz": 3},
			currentKeys: []string{"TestFoo", "TestQux"},
			want:        map[string]int{"TestFoo": 2, "TestQux": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateFailureHistory(tt.history, tt.currentKeys)
			if !mapsEqual(tt.history, tt.want) {
				t.Errorf("UpdateFailureHistory() = %v, want %v", tt.history, tt.want)
			}
		})
	}
}

func testAnnotateWithPersistentFailureHints(t *testing.T) {
	tests := []struct {
		name      string
		failures  []string
		history   map[string]int
		threshold int
		want      []string
	}{
		{
			name: "add hint when count >= threshold",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
			history:   map[string]int{"TestFoo": 2},
			threshold: 2,
			want: []string{
				"--- FAIL: TestFoo (0.01s)",
				"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
			},
		},
		{
			name: "leave failure unchanged when count < threshold",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
			history:   map[string]int{"TestFoo": 1},
			threshold: 2,
			want: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
		},
		{
			name: "handle mixed test and contract failures",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
				"contract:add-works — file_contains failed: expected output not found",
			},
			history:   map[string]int{"TestFoo": 3, "contract:add-works": 1},
			threshold: 2,
			want: []string{
				"--- FAIL: TestFoo (0.01s)",
				"persistent-failure: TestFoo has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
				"contract:add-works — file_contains failed: expected output not found",
			},
		},
		{
			name: "no hint for failures not in history",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
			history:   map[string]int{},
			threshold: 2,
			want: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
		},
		{
			name: "multiple failures with selective hints",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
				"--- FAIL: TestBar (0.02s)",
				"--- FAIL: TestBaz (0.03s)",
			},
			history:   map[string]int{"TestFoo": 2, "TestBar": 5, "TestBaz": 1},
			threshold: 2,
			want: []string{
				"--- FAIL: TestFoo (0.01s)",
				"persistent-failure: TestFoo has failed 2 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
				"--- FAIL: TestBar (0.02s)",
				"persistent-failure: TestBar has failed 5 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
				"--- FAIL: TestBaz (0.03s)",
			},
		},
		{
			name: "contract failures with hints",
			failures: []string{
				"contract:add-works — file_contains failed: expected output not found",
				"contract:list-shows-items — output_matches failed: regex mismatch",
			},
			history:   map[string]int{"contract:add-works": 3, "contract:list-shows-items": 1},
			threshold: 2,
			want: []string{
				"contract:add-works — file_contains failed: expected output not found",
				"persistent-failure: contract:add-works has failed 3 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
				"contract:list-shows-items — output_matches failed: regex mismatch",
			},
		},
		{
			name:      "empty failures returns empty list",
			failures:  []string{},
			history:   map[string]int{"TestFoo": 5},
			threshold: 2,
			want:      []string{},
		},
		{
			name: "threshold of zero applies to all failures",
			failures: []string{
				"--- FAIL: TestFoo (0.01s)",
			},
			history:   map[string]int{"TestFoo": 0},
			threshold: 0,
			want: []string{
				"--- FAIL: TestFoo (0.01s)",
				"persistent-failure: TestFoo has failed 0 consecutive cycles — may indicate a bad test specification rather than an implementation bug",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnnotateWithPersistentHints(tt.failures, tt.history, tt.threshold)
			if !slices.Equal(got, tt.want) {
				t.Errorf("AnnotateWithPersistentHints() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mapsEqual compares two maps, handling nil maps
func mapsEqual(a, b map[string]int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
