//go:build contract

package contracts

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/test/testutil"
)

var (
	// gromitBinary is the path to the built gromit binary
	gromitBinary string

	// realGitPath is the path to the real git binary (not the fake)
	realGitPath string

	// fakesDir is the absolute path to the test/fakes directory
	fakesDir string

	// fixturesDir is the absolute path to the test/fixtures directory
	fixturesDir string
)

// TestMain builds the gromit binary once and resolves the real git path
func TestMain(m *testing.M) {
	// Build the gromit binary
	tmpDir, err := os.MkdirTemp("", "gromit-contract-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	gromitBinary = filepath.Join(tmpDir, "gromit")

	// Resolve real git path by looking in common locations
	// We need to find git before we manipulate PATH to include the fake
	realGitPath = testutil.FindRealGit()
	if realGitPath == "" {
		fmt.Fprintf(os.Stderr, "Failed to find real git binary\n")
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	// Validate tagged harness invocation context and resolve shared helper paths.
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get working directory: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}
	harnessCtx, err := testutil.ResolveTaggedHarnessContext(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid contract harness invocation context: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run via: go test -tags=contract ./test/contracts")
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}
	fakesDir = harnessCtx.FakesDir
	fixturesDir = harnessCtx.FixturesDir

	buildCmd := exec.Command("go", "build", "-o", gromitBinary, harnessCtx.CmdGromitDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
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

// testEnv holds the test environment configuration
type testEnv struct {
	// Dir is the test directory (temporary, cleaned up after test)
	Dir string

	// CallLog is the path to the file where fake CLIs record invocations
	CallLog string

	// BDStateFile is the path to the fake bd state file
	BDStateFile string

	// PATH is the modified PATH with fakes prepended
	PATH string

	// Env is the full environment to pass to gromit
	Env []string
}

// setupTestEnv creates a temporary test environment with:
// - A git repository initialized in a temp directory
// - Fake CLIs prepended to PATH
// - A call log file for recording CLI invocations
// - A bd state file for stateful bd fake
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gromit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		// Clean up any claude fail-once state files before removing the directory
		testutil.CleanupClaudeFailOnceStateFiles(tmpDir)
		os.RemoveAll(tmpDir)
	})

	// Initialize git repo in temp dir
	gitInit := exec.Command(realGitPath, "init")
	gitInit.Dir = tmpDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to initialize git repo: %v", err)
	}

	// Configure git user for commits
	gitConfig := exec.Command(realGitPath, "config", "user.email", "test@example.com")
	gitConfig.Dir = tmpDir
	if err := gitConfig.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}

	gitConfig = exec.Command(realGitPath, "config", "user.name", "Test User")
	gitConfig.Dir = tmpDir
	if err := gitConfig.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	// Create call log file
	callLog := filepath.Join(tmpDir, "call_log.txt")

	// Create bd state file
	bdStateFile := filepath.Join(tmpDir, ".fake_bd_state.json")

	// Build modified PATH with fakes prepended
	originalPath := os.Getenv("PATH")
	modifiedPath := fakesDir + string(os.PathListSeparator) + originalPath

	// Build environment
	env := os.Environ()
	env = testutil.ReplaceOrAppend(env, "PATH", modifiedPath)
	env = testutil.ReplaceOrAppend(env, "TEST_DIR", tmpDir)
	env = testutil.ReplaceOrAppend(env, "TEST_CALL_LOG", callLog)
	env = testutil.ReplaceOrAppend(env, "REAL_GIT", realGitPath)

	return &testEnv{
		Dir:         tmpDir,
		CallLog:     callLog,
		BDStateFile: bdStateFile,
		PATH:        modifiedPath,
		Env:         env,
	}
}

// runGromitWithEnv runs the gromit binary with the given arguments and environment.
// It returns stdout, stderr, and the exit code (or error if the command couldn't be run).
func runGromitWithEnv(env *testEnv, args ...string) (stdout, stderr string, exitCode int, err error) {
	return testutil.RunGromitWithStdin(gromitBinary, env.Dir, env.Env, "", args...)
}

// runGromitWithStdin runs the gromit binary with the given arguments and stdin input in the test environment.
// The stdin parameter is piped to the command's standard input for testing interactive commands.
func runGromitWithStdin(env *testEnv, stdin string, args ...string) (stdout, stderr string, exitCode int, err error) {
	return testutil.RunGromitWithStdin(gromitBinary, env.Dir, env.Env, stdin, args...)
}

// readCallLog reads the call log file and returns all recorded CLI invocations.
// Each line in the log is one invocation (e.g., "bd ready --json --limit 1").
func readCallLog(env *testEnv) ([]string, error) {
	f, err := os.Open(env.CallLog)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var calls []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		calls = append(calls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}

// filterCalls returns all calls from the call log that start with the given prefix.
// For example, filterCalls(env, "bd") returns all bd CLI invocations.
func filterCalls(env *testEnv, prefix string) ([]string, error) {
	calls, err := readCallLog(env)
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, call := range calls {
		trimmed := strings.TrimSpace(call)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == prefix {
			filtered = append(filtered, trimmed)
		}
	}

	return filtered, nil
}
