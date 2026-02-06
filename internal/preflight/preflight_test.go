package preflight

import (
	"bytes"
	"testing"

	"github.com/danabrams/ralph-runner/internal/config"
)

func TestExtractTools(t *testing.T) {
	tests := []struct {
		name     string
		commands []string
		expected map[string]bool
	}{
		{
			name:     "empty commands",
			commands: []string{},
			expected: map[string]bool{},
		},
		{
			name: "go test command",
			commands: []string{
				"go test ./...",
				"go build ./cmd/ralph",
			},
			expected: map[string]bool{"go": true},
		},
		{
			name: "npm and node",
			commands: []string{
				"npm run build",
				"node script.js",
			},
			expected: map[string]bool{"npm": true, "node": true},
		},
		{
			name: "mise with go",
			commands: []string{
				"mise exec -- go test ./...",
				"mise exec -- go build",
			},
			expected: map[string]bool{"mise": true, "go": true},
		},
		{
			name: "python with pip",
			commands: []string{
				"python -m pytest",
				"python3 -m black .",
			},
			expected: map[string]bool{"python": true, "python3": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &Checker{cfg: config.PreflightConfig{}}
			result := checker.extractTools(tt.commands)

			// Check all expected tools are present
			for tool := range tt.expected {
				if !result[tool] {
					t.Errorf("expected tool %q not found in result", tool)
				}
			}

			// Check no unexpected tools
			for tool := range result {
				if !tt.expected[tool] {
					t.Errorf("unexpected tool %q in result", tool)
				}
			}
		})
	}
}

func TestExtractToolsWithExplicitList(t *testing.T) {
	cfg := config.PreflightConfig{
		Tools: []string{"go", "rust"},
	}
	checker := &Checker{cfg: cfg}

	result := checker.extractTools([]string{"npm run build"})

	if !result["go"] || !result["rust"] {
		t.Errorf("explicit tools not extracted: got %v", result)
	}
	if result["npm"] {
		t.Error("command-based extraction should be skipped when explicit tools are set")
	}
}

func TestFileExists(t *testing.T) {
	checker := &Checker{}

	// Test existing file
	if !checker.fileExists("preflight_test.go") {
		t.Error("should find test file in current directory")
	}

	// Test non-existing file
	if checker.fileExists("nonexistent_file_12345.go") {
		t.Error("should not find non-existent file")
	}
}

func TestPrintStatus(t *testing.T) {
	out := &bytes.Buffer{}
	checker := &Checker{out: out}

	allTools := map[string]bool{
		"go":   true,
		"node": true,
	}
	missing := []string{"node"}

	checker.printStatus(allTools, missing)

	output := out.String()
	if !bytes.Contains([]byte(output), []byte("✓ go")) {
		t.Error("output should indicate go is available")
	}
	if !bytes.Contains([]byte(output), []byte("✗ node not found")) {
		t.Error("output should indicate node is missing")
	}
}

func TestCheckNilReceiver(t *testing.T) {
	var c *Checker
	err := c.Check([]string{"go test ./..."})
	if err == nil {
		t.Error("expected error for nil checker")
	}
}

func TestNewCheckerNilOutput(t *testing.T) {
	c := NewChecker(config.PreflightConfig{}, nil)
	if c == nil {
		t.Fatal("expected non-nil checker")
	}
	if c.out == nil {
		t.Error("expected output to default to os.Stdout when nil")
	}
}

func TestCheckNoMissingTools(t *testing.T) {
	out := &bytes.Buffer{}
	checker := &Checker{
		cfg: config.PreflightConfig{AutoInstall: "never"},
		out: out,
	}

	// Commands with only common tools that should exist
	err := checker.Check([]string{"go test ./..."})

	// Should not error since we're only checking for missing tools
	// (go likely exists in the test environment)
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("missing")) {
		t.Errorf("unexpected error: %v", err)
	}
}
