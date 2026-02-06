package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/ralph-runner/internal/bead"
)

func TestCheckExpectedOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name            string
		expectedOutputs []string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "empty list returns empty string",
			expectedOutputs: []string{},
			wantNotContains: []string{"Expected outputs:"},
		},
		{
			name:            "existing file shows checkmark",
			expectedOutputs: []string{testFile},
			wantContains:    []string{"Expected outputs:", "✓", testFile, "(exists)"},
		},
		{
			name:            "missing file shows X",
			expectedOutputs: []string{filepath.Join(tmpDir, "missing.go")},
			wantContains:    []string{"Expected outputs:", "✗", "missing.go", "(not found)"},
		},
		{
			name:            "mixed existing and missing files",
			expectedOutputs: []string{testFile, filepath.Join(tmpDir, "missing.go")},
			wantContains:    []string{"Expected outputs:", "✓", testFile, "✗", "missing.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkExpectedOutputs(tt.expectedOutputs)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\nGot: %s", want, got)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("output should not contain %q\nGot: %s", notWant, got)
				}
			}
		})
	}
}

func TestCheckExpectedOutputsEmpty(t *testing.T) {
	got := checkExpectedOutputs(nil)
	if got != "" {
		t.Errorf("nil input should return empty string, got %q", got)
	}
}

func TestBeadExpectedOutputsField(t *testing.T) {
	b := &bead.Bead{
		ID:              "test-123",
		Title:           "Test Bead",
		ExpectedOutputs: []string{"file1.go", "file2.go"},
	}

	if len(b.ExpectedOutputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(b.ExpectedOutputs))
	}

	if b.ExpectedOutputs[0] != "file1.go" {
		t.Errorf("Expected first output to be file1.go, got %s", b.ExpectedOutputs[0])
	}
}

func TestBeadExpectedOutputsJSON(t *testing.T) {
	jsonData := `{
		"id": "abc-123",
		"title": "Add feature",
		"expected_outputs": ["internal/foo/bar.go", "internal/foo/bar_test.go"]
	}`

	var b bead.Bead
	if err := json.Unmarshal([]byte(jsonData), &b); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if b.ID != "abc-123" {
		t.Errorf("Expected ID abc-123, got %s", b.ID)
	}

	if len(b.ExpectedOutputs) != 2 {
		t.Errorf("Expected 2 outputs, got %d", len(b.ExpectedOutputs))
	}

	if b.ExpectedOutputs[0] != "internal/foo/bar.go" {
		t.Errorf("Expected first output internal/foo/bar.go, got %s", b.ExpectedOutputs[0])
	}
}

func TestBeadExpectedOutputsJSONOmitted(t *testing.T) {
	jsonData := `{"id": "abc-123", "title": "Simple task"}`

	var b bead.Bead
	if err := json.Unmarshal([]byte(jsonData), &b); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if b.ExpectedOutputs != nil {
		t.Errorf("Expected nil ExpectedOutputs when omitted, got %v", b.ExpectedOutputs)
	}
}

func TestGetGitHead(t *testing.T) {
	// This test runs in the ralph-runner repo, so git HEAD should be available
	head, err := getGitHead()
	if err != nil {
		t.Fatalf("getGitHead() failed: %v", err)
	}

	if len(head) != 40 {
		t.Errorf("Expected 40-char SHA, got %d chars: %s", len(head), head)
	}
}

func TestNewRunnerNilConfig(t *testing.T) {
	r := NewRunner(nil, os.Stdout)
	if r != nil {
		t.Error("expected nil Runner for nil config")
	}
}

func TestRunNilRunner(t *testing.T) {
	var r *Runner
	err := r.Run(nil, 0, false)
	if err == nil {
		t.Error("expected error for nil runner")
	}
}

func TestStatusNilRunner(t *testing.T) {
	var r *Runner
	err := r.Status()
	if err == nil {
		t.Error("expected error for nil runner")
	}
}

func TestSelectModelNilBead(t *testing.T) {
	r := &Runner{}
	result := r.selectModel(nil)
	if result != "sonnet" {
		t.Errorf("expected 'sonnet' for nil bead, got %q", result)
	}
}

func TestShowPartialProgressNilBead(t *testing.T) {
	var buf strings.Builder
	r := &Runner{output: &buf}
	// Should not panic with nil bead
	r.showPartialProgress(nil, "abc123")
}

func TestGetGitDiffStatSameCommit(t *testing.T) {
	// Diffing a commit against itself should produce empty output
	head, err := getGitHead()
	if err != nil {
		t.Skip("git not available")
	}

	stat, err := getGitDiffStat(head)
	if err != nil {
		t.Fatalf("getGitDiffStat() failed: %v", err)
	}

	// When comparing working tree to current HEAD, there may be uncommitted changes
	// but at minimum, the function should not error
	_ = stat
}
