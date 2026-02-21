//go:build acceptance

package bead

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// newIsolatedClient creates a bd client that operates in a temp directory
// so tests don't pollute the real project's beads database.
func newIsolatedClient(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("bd", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("bd init not available: %v: %s", err, out)
	}
	return &Client{binary: "bd", Dir: dir}
}

// TestClientCreate tests the Create() method
func TestClientCreate(t *testing.T) {
	tests := []struct {
		name            string
		title           string
		priority        int
		labels          []string
		expectedOutputs []string
	}{
		{
			name:            "basic creation",
			title:           "Test task",
			priority:        1,
			labels:          []string{"label1"},
			expectedOutputs: []string{"file.go"},
		},
		{
			name:            "empty labels and outputs",
			title:           "Simple task",
			priority:        2,
			labels:          []string{},
			expectedOutputs: []string{},
		},
		{
			name:            "nil labels and outputs",
			title:           "Nil fields task",
			priority:        0,
			labels:          nil,
			expectedOutputs: nil,
		},
		{
			name:            "multiple labels",
			title:           "Complex task",
			priority:        0,
			labels:          []string{"spec:auth", "complexity:high", "priority:p0"},
			expectedOutputs: []string{"auth.go", "auth_test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			var gotAcceptance string
			c := &Client{
				RunFn: func(args ...string) (string, error) {
					gotArgs = append([]string(nil), args...)
					accIdx := -1
					for i, arg := range args {
						if arg == "--acceptance" {
							accIdx = i
							break
						}
					}
					if accIdx != -1 && accIdx+1 < len(args) {
						gotAcceptance = args[accIdx+1]
					}
					return `{"id":"task-001","title":"` + tt.title + `","priority":` + fmt.Sprintf("%d", tt.priority) + `,"issue_type":"task","status":"open"}`, nil
				},
			}

			_, err := c.Create(tt.title, tt.priority, tt.labels, tt.expectedOutputs)
			if err != nil {
				t.Fatalf("Create() unexpected error: %v", err)
			}
			wantBase := []string{"create", tt.title, "--priority", fmt.Sprintf("%d", tt.priority), "--json"}
			if !hasSubsequence(gotArgs, wantBase) {
				t.Errorf("Create() args = %v, want subsequence %v", gotArgs, wantBase)
			}
			for _, label := range tt.labels {
				if !hasSubsequence(gotArgs, []string{"--label", label}) {
					t.Errorf("Create() missing label flag for %q in args %v", label, gotArgs)
				}
			}
			if len(tt.expectedOutputs) > 0 {
				wantAcceptance := strings.Join(tt.expectedOutputs, "\n")
				if !hasSubsequence(gotArgs, []string{"--acceptance"}) {
					t.Errorf("Create() missing --acceptance flag in args %v", gotArgs)
				}
				if gotAcceptance != wantAcceptance {
					t.Errorf("Create() acceptance = %q, want %q", gotAcceptance, wantAcceptance)
				}
			}
		})
	}
}

// TestClientCreateWithParent tests the CreateWithParent() method
func TestClientCreateWithParent(t *testing.T) {
	tests := []struct {
		name               string
		title              string
		priority           int
		labels             []string
		expectedOutputs    []string
		parentID           string
		shouldValidateFail bool
	}{
		{
			name:            "create with valid parent",
			title:           "Sub-task",
			priority:        1,
			labels:          []string{"label1"},
			expectedOutputs: []string{},
			parentID:        "parent-123",
		},
		{
			name:            "create with empty parent",
			title:           "Standalone task",
			priority:        1,
			labels:          []string{},
			expectedOutputs: []string{},
			parentID:        "",
		},
		{
			name:               "invalid parent ID with spaces",
			title:              "Task",
			priority:           1,
			parentID:           "parent 123",
			shouldValidateFail: true,
		},
		{
			name:               "invalid parent ID with shell chars",
			title:              "Task",
			priority:           1,
			parentID:           "parent; rm -rf /",
			shouldValidateFail: true,
		},
		{
			name:               "parent ID too long",
			title:              "Task",
			priority:           1,
			parentID:           strings.Repeat("a", maxIDLength+1),
			shouldValidateFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			c := &Client{
				RunFn: func(args ...string) (string, error) {
					gotArgs = append([]string(nil), args...)
					return `{"id":"task-001","title":"` + tt.title + `","priority":` + fmt.Sprintf("%d", tt.priority) + `,"issue_type":"task","status":"open"}`, nil
				},
			}
			_, err := c.CreateWithParent(tt.title, tt.priority, tt.labels, tt.expectedOutputs, tt.parentID)

			if tt.shouldValidateFail {
				if err == nil {
					t.Errorf("CreateWithParent() expected validation error for %q", tt.parentID)
				}
				if !strings.Contains(err.Error(), "invalid parent ID") {
					t.Errorf("CreateWithParent() expected 'invalid parent ID' error, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateWithParent() unexpected error: %v", err)
			}
			if tt.parentID != "" && !hasSubsequence(gotArgs, []string{"--parent", tt.parentID}) {
				t.Errorf("CreateWithParent() args missing parent %q: %v", tt.parentID, gotArgs)
			}
		})
	}
}

// TestClientCreateWithParentInheritance tests that Create() delegates to CreateWithParent
func TestClientCreateInheritsCreateWithParent(t *testing.T) {
	var createArgs []string
	createClient := &Client{
		RunFn: func(args ...string) (string, error) {
			createArgs = append([]string(nil), args...)
			return `{"id":"task-001","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}
	var parentArgs []string
	parentClient := &Client{
		RunFn: func(args ...string) (string, error) {
			parentArgs = append([]string(nil), args...)
			return `{"id":"task-002","title":"Test","priority":1,"issue_type":"task","status":"open"}`, nil
		},
	}

	_, err1 := createClient.Create("Test", 1, []string{}, []string{})
	_, err2 := parentClient.CreateWithParent("Test", 1, []string{}, []string{}, "")
	if err1 != nil || err2 != nil {
		t.Fatalf("Create() / CreateWithParent(\"\") unexpected errors: %v / %v", err1, err2)
	}
	if strings.Join(createArgs, " ") != strings.Join(parentArgs, " ") {
		t.Errorf("Create() args and CreateWithParent(\"\") args differ: %v vs %v", createArgs, parentArgs)
	}
}

// TestClientCreateWithDeps tests CreateWithDepsAndDescription with multiple dependencies
func TestClientCreateWithDeps(t *testing.T) {
	tests := []struct {
		name               string
		title              string
		priority           int
		labels             []string
		expectedOutputs    []string
		dependencies       []string
		description        string
		shouldValidateFail bool
		expectedErrMsg     string
	}{
		{
			name:            "create with no dependencies",
			title:           "Root task",
			priority:        1,
			labels:          []string{"label1"},
			expectedOutputs: []string{},
			dependencies:    []string{},
			description:     "Creates bead with no dependencies",
		},
		{
			name:            "create with single dependency",
			title:           "Sub-task",
			priority:        1,
			labels:          []string{},
			expectedOutputs: []string{},
			dependencies:    []string{"dep-1"},
			description:     "Creates bead with one dependency",
		},
		{
			name:            "create with multiple dependencies",
			title:           "Dependent task",
			priority:        1,
			labels:          []string{},
			expectedOutputs: []string{},
			dependencies:    []string{"dep-1", "dep-2", "dep-3"},
			description:     "Creates bead with multiple dependencies",
		},
		{
			name:               "invalid dependency ID with spaces",
			title:              "Task",
			priority:           1,
			dependencies:       []string{"dep 1"},
			shouldValidateFail: true,
			expectedErrMsg:     "invalid dependency ID",
		},
		{
			name:               "invalid dependency ID with shell chars",
			title:              "Task",
			priority:           1,
			dependencies:       []string{"dep-1", "dep; rm -rf /"},
			shouldValidateFail: true,
			expectedErrMsg:     "invalid dependency ID",
		},
		{
			name:               "dependency ID too long",
			title:              "Task",
			priority:           1,
			dependencies:       []string{strings.Repeat("a", maxIDLength+1)},
			shouldValidateFail: true,
			expectedErrMsg:     "invalid dependency ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgs []string
			c := &Client{
				RunFn: func(args ...string) (string, error) {
					gotArgs = append([]string(nil), args...)
					return `{"id":"task-001","title":"` + tt.title + `","priority":` + fmt.Sprintf("%d", tt.priority) + `,"issue_type":"task","status":"open"}`, nil
				},
			}
			_, err := c.CreateWithDepsAndDescription(tt.title, tt.priority, tt.labels, tt.expectedOutputs, tt.dependencies, tt.description)

			if tt.shouldValidateFail {
				if err == nil {
					t.Errorf("CreateWithDepsAndDescription() expected validation error")
				}
				if !strings.Contains(err.Error(), tt.expectedErrMsg) {
					t.Errorf("CreateWithDepsAndDescription() expected error containing %q, got: %v", tt.expectedErrMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateWithDepsAndDescription() unexpected error: %v", err)
			}
			if len(tt.dependencies) > 0 {
				wantDeps := strings.Join(tt.dependencies, ",")
				if !hasSubsequence(gotArgs, []string{"--deps", wantDeps}) {
					t.Errorf("CreateWithDepsAndDescription() args missing deps %q: %v", wantDeps, gotArgs)
				}
			}
		})
	}
}
