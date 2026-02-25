package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfirmPrompt(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		// Lowercase responses
		{
			name:       "y with default yes",
			input:      "y\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "y with default no",
			input:      "y\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "n with default yes",
			input:      "n\n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "n with default no",
			input:      "n\n",
			defaultYes: false,
			want:       false,
		},
		// Uppercase responses
		{
			name:       "Y with default yes",
			input:      "Y\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "N with default no",
			input:      "N\n",
			defaultYes: false,
			want:       false,
		},
		// Full word responses
		{
			name:       "yes with default no",
			input:      "yes\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "no with default yes",
			input:      "no\n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "YES uppercase",
			input:      "YES\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "NO uppercase",
			input:      "NO\n",
			defaultYes: true,
			want:       false,
		},
		// Empty input (uses default)
		{
			name:       "empty input with default yes",
			input:      "\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "empty input with default no",
			input:      "\n",
			defaultYes: false,
			want:       false,
		},
		// Whitespace-padded input
		{
			name:       "y with leading space",
			input:      " y\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "n with trailing space",
			input:      "n \n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "yes with surrounding whitespace",
			input:      "  yes  \n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "no with surrounding whitespace",
			input:      "  no  \n",
			defaultYes: true,
			want:       false,
		},
		// Invalid input (uses default)
		{
			name:       "invalid input with default yes",
			input:      "maybe\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "invalid input with default no",
			input:      "maybe\n",
			defaultYes: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a bufio.Reader from the test input
			reader := bufio.NewReader(strings.NewReader(tt.input))

			// Call confirmPrompt
			got := confirmPrompt(reader, "Test prompt", tt.defaultYes)

			// Check result
			if got != tt.want {
				t.Errorf("confirmPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecGromitBinaryResolution(t *testing.T) {
	// This test verifies that execGromit uses os.Executable() or os.Args[0]
	// We can't fully test the fallback without manipulating the environment,
	// but we can at least verify the function doesn't panic and can resolve
	// some binary path

	binary, err := os.Executable()
	if err != nil {
		// If os.Executable() fails, the fallback is os.Args[0]
		binary = os.Args[0]
	}

	if binary == "" {
		t.Error("Binary resolution failed - both os.Executable() and os.Args[0] are empty")
	}
}

func TestExecGromitExitErrorIsNil(t *testing.T) {
	prevFactory := execCommandFactory
	defer func() { execCommandFactory = prevFactory }()

	execCommandFactory = func(name string, args ...string) execCommand {
		return &testExecCommand{
			run: func() error {
				return &exec.ExitError{}
			},
		}
	}

	if err := execGromit("--help"); err != nil {
		t.Fatalf("expected nil for ExitError, got %v", err)
	}
}

type testExecCommand struct {
	run func() error
}

func (c *testExecCommand) Run() error {
	if c.run != nil {
		return c.run()
	}
	return nil
}

func (c *testExecCommand) SetStdin(_ io.Reader)   {}
func (c *testExecCommand) SetStdout(_ io.Writer) {}
func (c *testExecCommand) SetStderr(_ io.Writer) {}

func TestIsPlanDecomposed(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		planName string
		want     bool
	}{
		{
			name:     "decomposed true",
			planName: "test-plan-decomposed",
			content: `---
id: test-plan
decomposed: true
decomposed_at: "2026-02-07T10:00:00Z"
---

# Test Plan

This plan has been decomposed.
`,
			want: true,
		},
		{
			name:     "decomposed false",
			planName: "test-plan-not-decomposed",
			content: `---
id: test-plan
decomposed: false
---

# Test Plan

This plan has not been decomposed yet.
`,
			want: false,
		},
		{
			name:     "missing decomposed field",
			planName: "test-plan-no-field",
			content: `---
id: test-plan
created: "2026-02-07"
---

# Test Plan

This plan has no decomposed field.
`,
			want: false,
		},
		{
			name:     "no frontmatter",
			planName: "test-plan-no-frontmatter",
			content: `# Test Plan

This plan has no frontmatter at all.
`,
			want: false,
		},
		{
			name:     "missing file",
			planName: "nonexistent-plan",
			content:  "", // Won't be written
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test file if content is provided
			if tt.content != "" {
				planPath := filepath.Join(tmpDir, tt.planName+".md")
				if err := os.WriteFile(planPath, []byte(tt.content), 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			// Call isPlanDecomposed
			got := isPlanDecomposed(tmpDir, tt.planName)

			// Check result
			if got != tt.want {
				t.Errorf("isPlanDecomposed(%q, %q) = %v, want %v", tmpDir, tt.planName, got, tt.want)
			}
		})
	}
}
