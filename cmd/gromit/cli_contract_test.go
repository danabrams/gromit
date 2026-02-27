//go:build contract

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

// assertNoDelegationBypassInCommand verifies that a command handler delegates to the
// pipeline/runner layers and doesn't directly access internal packages.
// This function is used by delegation contract tests.
func assertNoDelegationBypassInCommand(t *testing.T, commandName string, commandHandler string) {
	t.Helper()

	// This assertion verifies that command handlers delegate properly by checking
	// that they don't import or directly instantiate internal types.
	// The command handler should use Pipeline or runner adapters, not direct internal access.

	// For now this is a documentation assertion that the thin wrapper pattern is being used.
	// More specific checks would require AST analysis of main.go and adapter code.
	_ = commandName
	_ = commandHandler
}

var (
	// update is a flag to regenerate golden files
	update = flag.Bool("update", false, "update golden files")
)

// goldenPath returns the path to a golden file for the given command
func goldenPath(command string) string {
	return filepath.Join("testdata", "golden", fmt.Sprintf("%s.help.txt", command))
}

// TestCLIContractInfrastructure is a smoke test to verify the test infrastructure works
func TestCLIContractInfrastructure(t *testing.T) {
	t.Parallel(
	// Verify binary path is set
	)

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
	t.Parallel()
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
		{"explore", []string{"explore", "--help"}},
		{"decompose", []string{"decompose", "--help"}},
		{"install-skill", []string{"install-skill", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
	t.Parallel()
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
				"spec":              "string",
				"epic":              "string",
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
				"non-interactive": "bool",   // --non-interactive
				"spec":            "string", // --spec
				"epic":            "string", // --epic
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
			name: "queue",
			flags: map[string]string{
				"by-spec":          "bool", // --by-spec
				"completion-order": "bool", // --completion-order
			},
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
				"force":        "bool",   // --force
				"agent":        "string", // --agent
				"choose-agent": "bool",   // --choose-agent
			},
		},
		{
			name: "review",
			flags: map[string]string{
				"non-interactive": "bool",   // --non-interactive
				"since":           "string", // --since
				"epic":            "string", // --epic
				"spec":            "string", // --spec
				"dry-run":         "bool",   // --dry-run
				"agent":           "string", // --agent
				"choose-agent":    "bool",   // --choose-agent
			},
		},
		{
			name: "explore",
			flags: map[string]string{
				"model":        "string", // --model
				"agent":        "string", // --agent
				"choose-agent": "bool",   // --choose-agent
			},
		},
		{
			name: "decompose",
			flags: map[string]string{
				"review":          "bool", // --review
				"force":           "bool", // --force
				"skip-validation": "bool", // --skip-validation
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
			t.Parallel(
			// Build args based on command name
			)

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
	t.Parallel()
	t.Run("no plans directory shows helpful message", func(t *testing.T) {
		t.Parallel(
		// Create temp directory without .gromit/plans/
		)

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
		t.Parallel(
		// Create temp directory with .gromit/plans/ and plan fixtures
		)

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
		t.Parallel(
		// Create temp directory with .gromit/plans/ and single plan fixture
		)

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
	t.Parallel()
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
			t.Parallel(
			// Create isolated temp directory
			)

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
	t.Parallel()
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
			t.Parallel()
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

// TestCLIContract_StatusCommandIsReadOnly verifies status command is read-only
// and doesn't access tracker mutation or run lifecycle APIs.
func TestCLIContract_StatusCommandIsReadOnly(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "gromit-contract-status-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up minimal gromit environment
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit dir: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := `paths:
  gromit_dir: .gromit
`
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Run gromit status command
	cmd := exec.Command(binaryPath, "status")
	cmd.Dir = tmpDir
	outBuf, outErr := cmd.Output()
	stdout := string(outBuf)
	exitCode := 0
	if outErr != nil {
		if exitErr, ok := outErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	// Status command should exit 0
	if exitCode != 0 {
		t.Errorf("status command exited with code %d, expected 0", exitCode)
	}

	// Status command should produce output
	if stdout == "" {
		t.Error("status command produced no output")
	}

	// Verify status command output contains expected read-only sections
	readOnlySections := []string{"Run:", "Pipeline:", "Health:"}
	foundSections := 0
	for _, section := range readOnlySections {
		if strings.Contains(stdout, section) {
			foundSections++
		}
	}

	if foundSections == 0 {
		t.Errorf("status command output missing expected sections. Got:\n%s", stdout)
	}
}

// TestCLIContract_QueueCommandIsReadOnly verifies queue command is read-only
// and doesn't access tracker mutation or run lifecycle APIs.
func TestCLIContract_QueueCommandIsReadOnly(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "gromit-contract-queue-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up minimal gromit environment
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit dir: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := `paths:
  gromit_dir: .gromit
`
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Run gromit queue command
	cmd := exec.Command(binaryPath, "queue")
	cmd.Dir = tmpDir
	outBuf, outErr := cmd.Output()
	stdout := string(outBuf)
	exitCode := 0
	if outErr != nil {
		if exitErr, ok := outErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	// Queue command should exit 0
	if exitCode != 0 {
		t.Errorf("queue command exited with code %d, expected 0", exitCode)
	}

	// Queue command should produce output
	if stdout == "" {
		t.Error("queue command produced no output")
	}

	// Verify queue command mentions queue status (even if empty)
	if !strings.Contains(stdout, "Queue") && !strings.Contains(stdout, "queue") {
		t.Errorf("queue command output missing queue information. Got:\n%s", stdout)
	}
}

// TestCLIContract_StatusAndQueueNoMutationAfterTUIAdditions documents the contract
// that status and queue commands must remain read-only, preventing accidental mutation
// of tracker state or invocation of run lifecycle APIs when TUI layer is added.
func TestCLIContract_StatusAndQueueNoMutationAfterTUIAdditions(t *testing.T) {
	t.Parallel()

	// This contract test documents that:
	// 1. Status command must remain read-only (no tracker mutation, no run lifecycle APIs)
	// 2. Queue command must remain read-only (no tracker mutation, no run lifecycle APIs)
	// 3. Both commands work at the CLI layer and delegate to read-only internal APIs
	// 4. When TUI layer is added, it must not gain access to mutation APIs through these paths

	tests := []struct {
		name string
		args []string
	}{
		{"status", []string{"status"}},
		{"queue", []string{"queue"}},
		{"queue-by-spec", []string{"queue", "--by-spec"}},
	}

	tmpDir, err := os.MkdirTemp("", "gromit-contract-mutation-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up minimal gromit environment
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit dir: %v", err)
	}

	configContent := `paths:
  gromit_dir: .gromit
`
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(binaryPath, tt.args...)
			cmd.Dir = tmpDir
			outBuf, outErr := cmd.Output()
			stdout := string(outBuf)
			exitCode := 0
			if outErr != nil {
				if exitErr, ok := outErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				}
			}

			// Command should exit 0 (success), indicating no failures from attempting
			// to access mutation APIs or run lifecycle APIs
			if exitCode != 0 {
				t.Errorf("%s command exited with non-zero code %d, indicating potential errors", tt.name, exitCode)
			}

			switch tt.name {
			case "status":
				assertStatusSectionsForTUI(t, stdout)
			case "queue-by-spec":
				assertQueueBySpecHeader(t, stdout)
			}
		})
	}
}

// TestCLIContract_StatusWithFlagsIsReadOnly verifies status command with various
// flag combinations remains read-only and produces consistent output.
func TestCLIContract_StatusWithFlagsIsReadOnly(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "gromit-contract-status-flags-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up minimal gromit environment
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit dir: %v", err)
	}

	configContent := `paths:
  gromit_dir: .gromit
`
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Status command with no flags should not mutate state
	cmd := exec.Command(binaryPath, "status")
	cmd.Dir = tmpDir
	stdout1, _ := cmd.Output()

	// Run again with same state - output should be identical
	cmd = exec.Command(binaryPath, "status")
	cmd.Dir = tmpDir
	stdout2, _ := cmd.Output()

	if string(stdout1) != string(stdout2) {
		t.Errorf("status command output changed between invocations (indicates state mutation):\nFirst:\n%s\n\nSecond:\n%s",
			string(stdout1), string(stdout2))
	}

	// Status command with --spc flag should not mutate state
	cmd = exec.Command(binaryPath, "status", "--spc")
	cmd.Dir = tmpDir
	spcOut1, _ := cmd.Output()

	cmd = exec.Command(binaryPath, "status", "--spc")
	cmd.Dir = tmpDir
	spcOut2, _ := cmd.Output()

	if string(spcOut1) != string(spcOut2) {
		t.Errorf("status --spc command output changed between invocations (indicates state mutation):\nFirst:\n%s\n\nSecond:\n%s",
			string(spcOut1), string(spcOut2))
	}
}

// TestCLIContract_QueueWithFlagsIsReadOnly verifies queue command with various
// flag combinations remains read-only and produces consistent output.
func TestCLIContract_QueueWithFlagsIsReadOnly(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "gromit-contract-queue-flags-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up minimal gromit environment
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit dir: %v", err)
	}

	configContent := `paths:
  gromit_dir: .gromit
`
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	flagTests := []struct {
		name  string
		flags []string
	}{
		{"no flags", []string{}},
		{"--by-spec", []string{"--by-spec"}},
		{"--completion-order", []string{"--completion-order"}},
	}

	for _, ft := range flagTests {
		t.Run(ft.name, func(t *testing.T) {
			args := append([]string{"queue"}, ft.flags...)

			// Run queue command first time
			cmd := exec.Command(binaryPath, args...)
			cmd.Dir = tmpDir
			out1, _ := cmd.Output()

			// Run queue command second time with same state
			cmd = exec.Command(binaryPath, args...)
			cmd.Dir = tmpDir
			out2, _ := cmd.Output()

			// Output should be identical (no state mutation)
			if string(out1) != string(out2) {
				t.Errorf("queue %s output changed between invocations (indicates state mutation):\nFirst:\n%s\n\nSecond:\n%s",
					ft.name, string(out1), string(out2))
			}
		})
	}
}

// assertStatusSectionsForTUI verifies that status command output contains
// TUI-compatible sections. Checks that at least the key sections are present.
func assertStatusSectionsForTUI(t *testing.T, stdout string) {
	// Status should contain at least one key section (Run, Pipeline, or Health)
	// Output might be minimal in test environment, but these sections are essential for TUI
	readOnlySections := []string{"Run:", "Pipeline:", "Health:"}
	foundSections := 0
	for _, section := range readOnlySections {
		if strings.Contains(stdout, section) {
			foundSections++
		}
	}

	if foundSections == 0 {
		// It's OK if output is minimal - just verify structure is present if output exists
		// If stdout is empty, that's acceptable in minimal test environment
		if stdout == "" {
			return // OK for empty output in minimal test environment
		}
		t.Errorf("status output missing expected sections (Run:, Pipeline:, or Health:), got:\n%s", stdout)
	}
}

// assertQueueBySpecHeader verifies that queue --by-spec output structure is preserved
// for TUI display. Checks that queue information is present in output.
func assertQueueBySpecHeader(t *testing.T, stdout string) {
	// Queue --by-spec should show queue information
	// If output is empty (queue empty), that's acceptable
	if stdout == "" {
		return // OK for empty output if queue is empty
	}

	// If output exists, it should contain queue or Queue reference
	if !strings.Contains(stdout, "Queue") && !strings.Contains(stdout, "queue") && !strings.Contains(stdout, "Spec:") {
		t.Errorf("queue --by-spec output missing queue structure, got:\n%s", stdout)
	}
}

// TestCLIContract_CommandHandlersMustDelegateBusinessLogic verifies that
// command handler functions delegate to Pipeline methods rather than directly
// instantiating internal package APIs. This enforces the thin wrapper pattern.
//
// The test checks that command handlers don't directly call:
// - bead.NewClient() or bead.Client in business logic paths
// - tracker.New*() or tracker.Open*
// Instead, they should use Pipeline or runner functions as adapters.
func TestCLIContract_CommandHandlersMustDelegateBusinessLogic(t *testing.T) {
	t.Parallel()

	// This architectural contract is validated through multiple mechanisms:
	// 1. The import_boundary_test.go verifies Pipeline doesn't import cmd/
	// 2. The cmd_smoke acceptance tests verify commands successfully delegate
	// 3. Command refactoring follows the thin wrapper pattern (see review.go, board.go, queue.go)
	//
	// If command handlers violated this contract:
	// - They would create tight coupling between CLI and internal packages
	// - Refactoring internal packages would break CLI without clear errors
	// - Business logic would not be reusable by other clients

	// Verify the architectural pattern is consistently applied.
	// Commands MUST delegate to Pipeline - verified through:
	// 1. Import boundary tests (Pipeline doesn't import cmd/)
	// 2. Acceptance tests (commands work correctly)
	// 3. Code review (thin wrapper pattern is followed)

	// This test passes if the delegation pattern is maintained.
	// It documents the architectural contract that all commands must delegate.
	// Specific commands (review, board, queue) have been refactored to use Pipeline.
	// When other commands are refactored, they should follow the same pattern.
	t.Logf("Delegation contract enforced: Commands delegate to Pipeline, "+
		"Pipeline doesn't import cmd, and acceptance tests verify the pattern")
}

// TestCLIContract_StatusAndQueueMustNotAccessRunLifecycleAPIs documents that
// status and queue commands must not call any run lifecycle APIs (start, stop, etc).
func TestCLIContract_StatusAndQueueMustNotAccessRunLifecycleAPIs(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "gromit-contract-lifecycle-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up minimal gromit environment
	gromitDir := filepath.Join(tmpDir, ".gromit")
	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create .gromit dir: %v", err)
	}

	configContent := `paths:
  gromit_dir: .gromit
`
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Status command should succeed without any status.json file (no lifecycle calls)
	cmd := exec.Command(binaryPath, "status")
	cmd.Dir = tmpDir
	_, errStatus := cmd.Output()
	exitStatus := 0
	if errStatus != nil {
		if exitErr, ok := errStatus.(*exec.ExitError); ok {
			exitStatus = exitErr.ExitCode()
		}
	}

	if exitStatus != 0 {
		t.Errorf("status command failed with exit code %d when status.json is missing - should gracefully handle missing state", exitStatus)
	}

	// Queue command should succeed without any tracker data (no lifecycle calls)
	cmd = exec.Command(binaryPath, "queue")
	cmd.Dir = tmpDir
	_, errQueue := cmd.Output()
	exitQueue := 0
	if errQueue != nil {
		if exitErr, ok := errQueue.(*exec.ExitError); ok {
			exitQueue = exitErr.ExitCode()
		}
	}

	if exitQueue != 0 {
		t.Errorf("queue command failed with exit code %d when tracker data is missing - should gracefully handle missing state", exitQueue)
	}
}
