package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

	cmd := exec.Command(binaryPath, args...)

	outBuf, outErr := cmd.Output()
	exitCode = 0

	if outErr != nil {
		if exitErr, ok := outErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run gromit %v: %v", args, outErr)
		}
	}

	stdout = string(outBuf)

	return stdout, stderr, exitCode
}

// runGromitWithStdin executes the gromit binary with the given arguments and stdin input,
// returning stdout, stderr, and exit code. This is useful for testing interactive commands.
func runGromitWithStdin(t *testing.T, stdin string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdin = strings.NewReader(stdin)

	outBuf, outErr := cmd.Output()
	exitCode = 0

	if outErr != nil {
		if exitErr, ok := outErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("Failed to run gromit %v with stdin: %v", args, outErr)
		}
	}

	stdout = string(outBuf)

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
			name:  "refine",
			flags: map[string]string{},
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
