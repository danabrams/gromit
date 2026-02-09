package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

var (
	// binaryPath is the path to the built gromit binary
	binaryPath string

	// update is a flag to regenerate golden files
	update = flag.Bool("update", false, "update golden files")
)

// TestMain builds the gromit binary once before running tests
func TestMain(m *testing.M) {
	// Parse flags
	flag.Parse()

	// Create a temporary directory for the test binary
	tmpDir, err := os.MkdirTemp("", "gromit-cli-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	// Build the gromit binary
	binaryPath = filepath.Join(tmpDir, "gromit")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to build gromit binary: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	// Run the tests
	exitCode := m.Run()

	// Clean up before exit (os.Exit doesn't run defers)
	os.RemoveAll(tmpDir)
	os.Exit(exitCode)
}

// runGromit executes the gromit binary with the given arguments and returns stdout, stderr, and exit code
func runGromit(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(binaryPath, "", nil, "", args...)
	if err != nil {
		t.Fatalf("Failed to run gromit %v: %v", args, err)
	}

	return stdout, stderr, exitCode
}

// runGromitWithStdin executes the gromit binary with the given arguments and stdin input,
// returning stdout, stderr, and exit code. This is useful for testing interactive commands.
func runGromitWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(binaryPath, "", nil, stdin, args...)
	if err != nil {
		t.Fatalf("Failed to run gromit %v with stdin: %v", args, err)
	}

	return stdout, stderr, exitCode
}

// goldenPath returns the path to a golden file for the given command
func goldenPath(command string) string {
	return filepath.Join("testdata", "golden", fmt.Sprintf("%s.help.txt", command))
}

// TestCLIContractInfrastructure is a smoke test to verify the test infrastructure works
func TestCLIContractInfrastructure(t *testing.T) {
	// Verify binary path is set
	if binaryPath == "" {
		t.Fatal("binaryPath is empty - TestMain did not run")
	}

	// Verify binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("binary does not exist at %s: %v", binaryPath, err)
	}

	// Verify runGromit works
	stdout, stderr, exitCode := runGromit(t, "--help")
	if exitCode != 0 {
		t.Errorf("gromit --help exited with code %d, stderr: %s", exitCode, stderr)
	}
	if stdout == "" {
		t.Error("gromit --help produced no output")
	}

	// Verify goldenPath works
	path := goldenPath("test")
	expected := filepath.Join("testdata", "golden", "test.help.txt")
	if path != expected {
		t.Errorf("goldenPath(test) = %s, want %s", path, expected)
	}
}

// TestCLIContract_HelpText verifies help output for all commands matches golden files
func TestCLIContract_HelpText(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"root", []string{"--help"}},
		{"run", []string{"run", "--help"}},
		{"init", []string{"init", "--help"}},
		{"status", []string{"status", "--help"}},
		{"retro", []string{"retro", "--help"}},
		{"add", []string{"add", "--help"}},
		{"backlog", []string{"backlog", "--help"}},
		{"backlog-delete", []string{"backlog", "delete", "--help"}},
		{"board", []string{"board", "--help"}},
		{"queue", []string{"queue", "--help"}},
		{"triage", []string{"triage", "--help"}},
		{"refine", []string{"refine", "--help"}},
		{"plan", []string{"plan", "--help"}},
		{"review", []string{"review", "--help"}},
		{"decompose", []string{"decompose", "--help"}},
		{"install-skill", []string{"install-skill", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := runGromit(t, tt.args...)

			// Help commands should exit with code 0
			if exitCode != 0 {
				t.Errorf("gromit %v exited with code %d, stderr: %s", tt.args, exitCode, stderr)
			}

			// Get golden file path
			golden := goldenPath(tt.name)

			// If -update flag is set, write the golden file
			if *update {
				dir := filepath.Dir(golden)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create golden directory: %v", err)
				}
				if err := os.WriteFile(golden, []byte(stdout), 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("Updated golden file: %s", golden)
				return
			}

			// Read golden file
			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v\nRun with -update flag to create it", golden, err)
			}

			// Compare output
			if stdout != string(expected) {
				t.Errorf("help output mismatch for %s\nRun with -update flag to update golden file\n\nGot:\n%s\n\nExpected:\n%s",
					tt.name, stdout, string(expected))
			}
		})
	}
}

// flagContract defines the expected flags for a command
type flagContract struct {
	name  string            // command name
	flags map[string]string // flag name -> flag type (bool, string, int, etc.)
}

// TestCLIContract_Flags verifies that commands have expected flags
func TestCLIContract_Flags(t *testing.T) {
	contracts := []flagContract{
		{
			name: "root",
			flags: map[string]string{
				"config": "string", // -c, --config
			},
		},
		{
			name: "run",
			flags: map[string]string{
				"max-iterations":    "int",  // -n, --max-iterations
				"dry-run":           "bool", // --dry-run
				"time-budget":       "int",  // -t, --time-budget
				"time-budget-hours": "int",  // -H, --time-budget-hours
			},
		},
		{
			name: "init",
			flags: map[string]string{
				"force": "bool", // -f, --force
			},
		},
		{
			name:  "status",
			flags: map[string]string{},
		},
		{
			name: "retro",
			flags: map[string]string{
				"non-interactive": "bool", // --non-interactive
			},
		},
		{
			name:  "add",
			flags: map[string]string{},
		},
		{
			name: "backlog",
			flags: map[string]string{
				"type":   "string", // --type
				"recent": "int",    // --recent
			},
		},
		{
			name:  "backlog-delete",
			flags: map[string]string{},
		},
		{
			name:  "board",
			flags: map[string]string{},
		},
		{
			name:  "queue",
			flags: map[string]string{},
		},
		{
			name:  "triage",
			flags: map[string]string{},
		},
		{
			name: "refine",
			flags: map[string]string{
				"agent":        "string", // --agent
				"choose-agent": "bool",   // --choose-agent
			},
		},
		{
			name: "plan",
			flags: map[string]string{
				"force": "bool", // --force
			},
		},
		{
			name: "review",
			flags: map[string]string{
				"non-interactive": "bool",   // --non-interactive
				"since":           "string", // --since
				"epic":            "string", // --epic
				"dry-run":         "bool",   // --dry-run
			},
		},
		{
			name: "decompose",
			flags: map[string]string{
				"review": "bool", // --review
				"force":  "bool", // --force
			},
		},
		{
			name: "install-skill",
			flags: map[string]string{
				"force": "bool", // -f, --force
			},
		},
	}

	for _, contract := range contracts {
		t.Run(contract.name, func(t *testing.T) {
			// Build args based on command name
			var args []string
			if contract.name == "root" {
				args = []string{"--help"}
			} else if contract.name == "backlog-delete" {
				args = []string{"backlog", "delete", "--help"}
			} else {
				args = []string{contract.name, "--help"}
			}

			stdout, stderr, exitCode := runGromit(t, args...)

			// Help commands should exit with code 0
			if exitCode != 0 {
				t.Errorf("gromit %v exited with code %d, stderr: %s", args, exitCode, stderr)
			}

			// Parse help output for flags
			actualFlags := parseFlagsFromHelp(stdout)

			// Check for missing flags
			for expectedFlag := range contract.flags {
				if !actualFlags[expectedFlag] {
					t.Errorf("Expected flag --%s is missing from %s command", expectedFlag, contract.name)
				}
			}

			// Check for unexpected flags (excluding global flags like help)
			for actualFlag := range actualFlags {
				// Skip global flags that appear on all commands
				if actualFlag == "help" {
					continue
				}
				// Skip the persistent config flag inherited from root
				if actualFlag == "config" && contract.name != "root" {
					continue
				}
				if _, expected := contract.flags[actualFlag]; !expected {
					t.Errorf("Unexpected flag --%s found in %s command (if this is intentional, update the contract)",
						actualFlag, contract.name)
				}
			}
		})
	}
}

// parseFlagsFromHelp extracts flag names from help output
func parseFlagsFromHelp(helpText string) map[string]bool {
	flags := make(map[string]bool)

	// Look for lines containing flags (e.g., "  -f, --force")
	// Flags are typically formatted as "  -x, --long-name" or "  --long-name"
	lines := strings.Split(helpText, "\n")
	for _, line := range lines {
		// Skip lines that don't look like flag definitions
		if !strings.Contains(line, "--") {
			continue
		}

		// Extract the long flag name (after --)
		// Format: "  -x, --flag-name type    description" or "      --flag-name type    description"
		parts := strings.Fields(line)
		for _, part := range parts {
			if strings.HasPrefix(part, "--") {
				// Remove leading -- and any trailing punctuation
				flagName := strings.TrimPrefix(part, "--")
				flagName = strings.TrimRight(flagName, ",:;.\"'")
				flags[flagName] = true
			}
		}
	}

	return flags
}

// TestCLIContract_DecomposePickerBehavior tests the interactive picker for decompose command
func TestCLIContract_DecomposePickerBehavior(t *testing.T) {
	t.Run("no plans directory shows helpful message", func(t *testing.T) {
		// Create temp directory without .gromit/plans/
		tmpDir, err := os.MkdirTemp("", "gromit-cli-decompose-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Set up minimal gromit environment (but no plans directory)
		gromitDir := filepath.Join(tmpDir, ".gromit")
		if err := os.MkdirAll(gromitDir, 0755); err != nil {
			t.Fatalf("failed to create .gromit dir: %v", err)
		}

		// Create minimal gromit.yaml
		configContent := `paths:
  gromit_dir: .gromit
  plans_dir: .gromit/plans
`
		configPath := filepath.Join(tmpDir, "gromit.yaml")
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Run gromit decompose with no arguments in this directory
		cmd := exec.Command(binaryPath, "decompose")
		cmd.Dir = tmpDir
		outBuf, outErr := cmd.Output()
		stdout := string(outBuf)
		exitCode := 0
		if outErr != nil {
			if exitErr, ok := outErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				t.Fatalf("Failed to run gromit decompose: %v", outErr)
			}
		}

		// Should exit 0
		if exitCode != 0 {
			t.Errorf("gromit decompose with no plans dir exited with code %d, want 0", exitCode)
		}

		// Should show helpful message
		if !strings.Contains(stdout, "No undecomposed plans found") {
			t.Errorf("expected helpful message about no plans, got: %s", stdout)
		}
		if !strings.Contains(stdout, "gromit plan") {
			t.Errorf("expected mention of 'gromit plan' command, got: %s", stdout)
		}
	})

	t.Run("with undecomposed plans shows picker", func(t *testing.T) {
		// Create temp directory with .gromit/plans/ and plan fixtures
		tmpDir, err := os.MkdirTemp("", "gromit-cli-decompose-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Set up gromit environment
		plansDir := filepath.Join(tmpDir, ".gromit", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}

		// Create minimal gromit.yaml
		configContent := `paths:
  gromit_dir: .gromit
  plans_dir: .gromit/plans
`
		configPath := filepath.Join(tmpDir, "gromit.yaml")
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Create undecomposed plan fixtures
		plan1 := `---
decomposed: false
---

# Add User Authentication

This is a plan for adding user authentication.
`
		plan1Path := filepath.Join(plansDir, "auth.md")
		if err := os.WriteFile(plan1Path, []byte(plan1), 0644); err != nil {
			t.Fatalf("failed to write plan1: %v", err)
		}

		plan2 := `---
decomposed: false
---

# Implement API Endpoints

This is a plan for implementing API endpoints.
`
		plan2Path := filepath.Join(plansDir, "api-endpoints.md")
		if err := os.WriteFile(plan2Path, []byte(plan2), 0644); err != nil {
			t.Fatalf("failed to write plan2: %v", err)
		}

		// Create already-decomposed plan (should not appear in picker without --force)
		plan3 := `---
decomposed: true
decomposed_at: "2026-02-07T10:00:00Z"
---

# Database Migration

This plan has already been decomposed.
`
		plan3Path := filepath.Join(plansDir, "database.md")
		if err := os.WriteFile(plan3Path, []byte(plan3), 0644); err != nil {
			t.Fatalf("failed to write plan3: %v", err)
		}

		// Run gromit decompose with no arguments and provide "1" as input (select first plan)
		cmd := exec.Command(binaryPath, "decompose")
		cmd.Dir = tmpDir
		cmd.Stdin = strings.NewReader("1\n")
		outBuf, outErr := cmd.Output()
		stdout := string(outBuf)

		// We expect this to fail because Claude/bd won't actually work in test env
		// But we can verify the picker was shown correctly
		if outErr != nil {
			// This is expected - the test will fail when trying to actually decompose
			// We just want to verify the picker output
		}

		// Verify picker was displayed
		if !strings.Contains(stdout, "Select a plan to decompose:") {
			t.Errorf("expected picker prompt, got: %s", stdout)
		}

		// Verify plan names are shown
		if !strings.Contains(stdout, "api-endpoints") {
			t.Errorf("expected 'api-endpoints' in picker, got: %s", stdout)
		}
		if !strings.Contains(stdout, "auth") {
			t.Errorf("expected 'auth' in picker, got: %s", stdout)
		}

		// Verify plan titles are shown
		if !strings.Contains(stdout, "Add User Authentication") {
			t.Errorf("expected 'Add User Authentication' title in picker, got: %s", stdout)
		}
		if !strings.Contains(stdout, "Implement API Endpoints") {
			t.Errorf("expected 'Implement API Endpoints' title in picker, got: %s", stdout)
		}

		// Verify already-decomposed plan is NOT shown (without --force)
		if strings.Contains(stdout, "database") || strings.Contains(stdout, "Database Migration") {
			t.Errorf("already-decomposed plan should not appear in picker without --force, got: %s", stdout)
		}

		// Verify "Decompose all" option is shown (since we have 2+ undecomposed plans)
		if !strings.Contains(stdout, "Decompose all") {
			t.Errorf("expected 'Decompose all' option with 2+ plans, got: %s", stdout)
		}
	})

	t.Run("with single undecomposed plan shows picker without decompose all", func(t *testing.T) {
		// Create temp directory with .gromit/plans/ and single plan fixture
		tmpDir, err := os.MkdirTemp("", "gromit-cli-decompose-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		// Set up gromit environment
		plansDir := filepath.Join(tmpDir, ".gromit", "plans")
		if err := os.MkdirAll(plansDir, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}

		// Create minimal gromit.yaml
		configContent := `paths:
  gromit_dir: .gromit
  plans_dir: .gromit/plans
`
		configPath := filepath.Join(tmpDir, "gromit.yaml")
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Create single undecomposed plan fixture
		plan := `---
decomposed: false
---

# Add Logging

This is a plan for adding logging.
`
		planPath := filepath.Join(plansDir, "logging.md")
		if err := os.WriteFile(planPath, []byte(plan), 0644); err != nil {
			t.Fatalf("failed to write plan: %v", err)
		}

		// Run gromit decompose with no arguments and provide "1" as input
		cmd := exec.Command(binaryPath, "decompose")
		cmd.Dir = tmpDir
		cmd.Stdin = strings.NewReader("1\n")
		outBuf, _ := cmd.Output()
		stdout := string(outBuf)

		// Verify picker was displayed
		if !strings.Contains(stdout, "Select a plan to decompose:") {
			t.Errorf("expected picker prompt, got: %s", stdout)
		}

		// Verify plan is shown
		if !strings.Contains(stdout, "logging") {
			t.Errorf("expected 'logging' in picker, got: %s", stdout)
		}

		// Verify "Decompose all" option is NOT shown (only 1 plan)
		if strings.Contains(stdout, "Decompose all") {
			t.Errorf("'Decompose all' should not appear with only 1 plan, got: %s", stdout)
		}
	})
}

// TestCLIContract_AddContextCapture verifies that multi-word context is captured correctly
func TestCLIContract_AddContextCapture(t *testing.T) {
	tests := []struct {
		name          string
		ideaText      string
		stdin         string // stdin input: context input + newlines
		wantContext   string
		wantInBacklog bool
	}{
		{
			name:          "multi-word context",
			ideaText:      "Add user authentication",
			stdin:         "this should work with the new auth system\n",
			wantContext:   "this should work with the new auth system",
			wantInBacklog: true,
		},
		{
			name:          "empty context",
			ideaText:      "Fix the bug",
			stdin:         "\n",
			wantContext:   "",
			wantInBacklog: true,
		},
		{
			name:          "single word context",
			ideaText:      "Refactor code",
			stdin:         "TDD\n",
			wantContext:   "TDD",
			wantInBacklog: true,
		},
		{
			name:          "context with punctuation",
			ideaText:      "Add logging",
			stdin:         "need this for debugging, ASAP!\n",
			wantContext:   "need this for debugging, ASAP!",
			wantInBacklog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create isolated temp directory
			tmpDir, err := os.MkdirTemp("", "gromit-add-context-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create minimal gromit.yaml
			configContent := `paths:
  gromit_dir: .gromit
`
			configPath := filepath.Join(tmpDir, "gromit.yaml")
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			// Run gromit add with context input via stdin
			stdout, stderr, exitCode, err := testutil.RunGromitWithStdin(
				binaryPath,
				tmpDir,
				nil,
				tt.stdin,
				"add", tt.ideaText,
			)
			if err != nil {
				t.Fatalf("failed to run gromit add: %v", err)
			}

			// Command should exit 0
			if exitCode != 0 {
				t.Errorf("gromit add exited with code %d, stderr: %s", exitCode, stderr)
			}

			// Verify success message
			if !strings.Contains(stdout, "Added to backlog") {
				t.Errorf("expected success message, got: %s", stdout)
			}

			// Verify backlog.jsonl was created and contains the idea
			backlogPath := filepath.Join(tmpDir, ".gromit", "backlog.jsonl")
			data, err := os.ReadFile(backlogPath)
			if err != nil {
				t.Fatalf("failed to read backlog.jsonl: %v", err)
			}

			// Parse the JSON line
			var idea struct {
				ID      string `json:"id"`
				Text    string `json:"text"`
				Context string `json:"context"`
			}
			if err := json.Unmarshal(data, &idea); err != nil {
				t.Fatalf("failed to unmarshal backlog entry: %v", err)
			}

			// Verify the context field matches expected value
			if idea.Context != tt.wantContext {
				t.Errorf("context mismatch:\ngot:  %q\nwant: %q", idea.Context, tt.wantContext)
			}

			// Verify the idea text is correct
			if idea.Text != tt.ideaText {
				t.Errorf("idea text mismatch:\ngot:  %q\nwant: %q", idea.Text, tt.ideaText)
			}
		})
	}
}

// TestCLIContract_ExitCodes verifies exit codes for various error conditions
func TestCLIContract_ExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantExit   int
		wantStderr string // substring that should appear in stderr
	}{
		// Help commands should exit 0
		{
			name:     "root help",
			args:     []string{"--help"},
			wantExit: 0,
		},
		{
			name:     "run help",
			args:     []string{"run", "--help"},
			wantExit: 0,
		},
		{
			name:     "init help",
			args:     []string{"init", "--help"},
			wantExit: 0,
		},
		{
			name:     "install-skill help",
			args:     []string{"install-skill", "--help"},
			wantExit: 0,
		},

		// Missing required arguments
		{
			name:       "add missing argument",
			args:       []string{"add"},
			wantExit:   1,
			wantStderr: "accepts 1 arg(s), received 0",
		},
		{
			name:       "backlog delete missing argument",
			args:       []string{"backlog", "delete"},
			wantExit:   1,
			wantStderr: "accepts 1 arg(s), received 0",
		},

		// Invalid flag values
		{
			name:       "run invalid max-iterations",
			args:       []string{"run", "--max-iterations=invalid"},
			wantExit:   1,
			wantStderr: "invalid argument",
		},
		{
			name:       "run invalid time-budget",
			args:       []string{"run", "--time-budget=invalid"},
			wantExit:   1,
			wantStderr: "invalid argument",
		},
		{
			name:       "run invalid time-budget-hours",
			args:       []string{"run", "--time-budget-hours=invalid"},
			wantExit:   1,
			wantStderr: "invalid argument",
		},
		{
			name:       "backlog invalid recent",
			args:       []string{"backlog", "--recent=invalid"},
			wantExit:   1,
			wantStderr: "invalid argument",
		},

		// Unknown commands
		{
			name:       "unknown command",
			args:       []string{"nonexistent"},
			wantExit:   1,
			wantStderr: "unknown command",
		},

		// Unknown flags
		{
			name:       "unknown flag",
			args:       []string{"--nonexistent"},
			wantExit:   1,
			wantStderr: "unknown flag",
		},
		{
			name:       "run unknown flag",
			args:       []string{"run", "--nonexistent"},
			wantExit:   1,
			wantStderr: "unknown flag",
		},
		{
			name:       "init unknown flag",
			args:       []string{"init", "--nonexistent"},
			wantExit:   1,
			wantStderr: "unknown flag",
		},
		{
			name:       "install-skill unknown flag",
			args:       []string{"install-skill", "--nonexistent"},
			wantExit:   1,
			wantStderr: "unknown flag",
		},

		// Short flag variations
		{
			name:     "run short max-iterations help",
			args:     []string{"run", "-h"},
			wantExit: 0,
		},
		{
			name:     "init short force help",
			args:     []string{"init", "-h"},
			wantExit: 0,
		},
		{
			name:     "install-skill short help",
			args:     []string{"install-skill", "-h"},
			wantExit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := runGromit(t, tt.args...)

			// Check exit code
			if exitCode != tt.wantExit {
				t.Errorf("gromit %v exited with code %d, want %d\nstdout: %s\nstderr: %s",
					tt.args, exitCode, tt.wantExit, stdout, stderr)
			}

			// Check stderr content if specified
			if tt.wantStderr != "" {
				if !strings.Contains(stderr, tt.wantStderr) && !strings.Contains(stdout, tt.wantStderr) {
					t.Errorf("gromit %v output missing expected text %q\nstdout: %s\nstderr: %s",
						tt.args, tt.wantStderr, stdout, stderr)
				}
			}
		})
	}
}
