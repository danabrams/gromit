package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestRunGromitWithStdin(t *testing.T) {
	t.Parallel()

	testScriptContent := `#!/bin/bash
cat
exit ${1:-0}
`
	tests := []struct {
		name         string
		useTmpDir    bool
		environ      []string
		stdin        string
		args         []string
		wantStdout   string
		wantExitCode int
		wantErr      bool
	}{
		{
			name:         "basic stdin echo",
			useTmpDir:    false,
			environ:      nil,
			stdin:        "hello world\n",
			args:         []string{"0"},
			wantStdout:   "hello world\n",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "non-zero exit code",
			useTmpDir:    false,
			environ:      nil,
			stdin:        "test input\n",
			args:         []string{"42"},
			wantStdout:   "test input\n",
			wantExitCode: 42,
			wantErr:      false,
		},
		{
			name:         "empty stdin",
			useTmpDir:    false,
			environ:      nil,
			stdin:        "",
			args:         []string{"0"},
			wantStdout:   "",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "with dir set",
			useTmpDir:    true,
			environ:      nil,
			stdin:        "test\n",
			args:         []string{"0"},
			wantStdout:   "test\n",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "with environ set",
			useTmpDir:    false,
			environ:      []string{"FOO=bar"},
			stdin:        "test\n",
			args:         []string{"0"},
			wantStdout:   "test\n",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "nonexistent binary",
			useTmpDir:    false,
			environ:      nil,
			stdin:        "test\n",
			args:         []string{},
			wantStdout:   "",
			wantExitCode: -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			testBinary := createTempShellScript(t, tmpDir, "test-echo", testScriptContent)
			if tt.name == "nonexistent binary" {
				testBinary = filepath.Join(tmpDir, "missing-binary")
			}
			dir := ""
			if tt.useTmpDir {
				dir = tmpDir
			}

			stdout, stderr, exitCode, err := RunGromitWithStdin(testBinary, dir, tt.environ, tt.stdin, tt.args...)

			if (err != nil) != tt.wantErr {
				t.Errorf("RunGromitWithStdin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if stdout != tt.wantStdout {
				t.Errorf("RunGromitWithStdin() stdout = %q, want %q", stdout, tt.wantStdout)
			}

			if exitCode != tt.wantExitCode {
				t.Errorf("RunGromitWithStdin() exitCode = %d, want %d", exitCode, tt.wantExitCode)
			}

			// For the non-existent binary test, we expect stderr to be empty since the error
			// occurs before the command runs
			if tt.name == "nonexistent binary" && stderr != "" {
				t.Errorf("RunGromitWithStdin() stderr = %q, want empty for pre-execution error", stderr)
			}
		})
	}
}

func createTempShellScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, name)
	// Write to a temp file then rename atomically to avoid ETXTBSY on Linux:
	// when parallel subtests write+exec concurrently, os.WriteFile may have
	// O_WRONLY open on an inode that another goroutine's fork() inherits.
	// The rename ensures the final path has no open write fd at exec time.
	f, err := os.CreateTemp(dir, name+".*.tmp")
	if err != nil {
		t.Fatalf("failed to create temp file for script %s: %v", name, err)
	}
	tmpPath := f.Name()
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(tmpPath)
		t.Fatalf("failed to write test shell script %s: %v", name, err)
	}
	if err := f.Chmod(0755); err != nil {
		f.Close()
		os.Remove(tmpPath)
		t.Fatalf("failed to chmod test shell script %s: %v", name, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		t.Fatalf("failed to close test shell script %s: %v", name, err)
	}
	if err := os.Rename(tmpPath, scriptPath); err != nil {
		os.Remove(tmpPath)
		t.Fatalf("failed to rename test shell script to %s: %v", scriptPath, err)
	}
	return scriptPath
}

func TestRunGromitWithStdin_EmptyDirNotSet(t *testing.T) {
	t.Parallel()

	// This test verifies that when dir is empty, cmd.Dir is not set
	// We can't directly observe cmd.Dir after execution, but we can verify
	// the command runs in the current directory by checking the working directory

	tmpDir := t.TempDir()

	// Write a script that prints the current working directory
	scriptContent := `#!/bin/bash
pwd
`
	testBinary := createTempShellScript(t, tmpDir, "pwd-script", scriptContent)

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Run with empty dir
	stdout, _, exitCode, err := RunGromitWithStdin(testBinary, "", nil, "")
	if err != nil {
		t.Fatalf("RunGromitWithStdin() error = %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("RunGromitWithStdin() exitCode = %d, want 0", exitCode)
	}

	// The output should be the current working directory
	gotDir := strings.TrimSpace(stdout)
	if gotDir != cwd {
		t.Errorf("With empty dir, command ran in %q, expected current directory %q", gotDir, cwd)
	}
}

func TestRunGromitWithStdin_DirIsSet(t *testing.T) {
	t.Parallel()

	// This test verifies that when dir is non-empty, cmd.Dir is set correctly

	tmpDir := t.TempDir()

	// Write a script that prints the current working directory
	scriptContent := `#!/bin/bash
pwd
`
	testBinary := createTempShellScript(t, tmpDir, "pwd-script", scriptContent)

	// Run with tmpDir set as dir
	stdout, _, exitCode, err := RunGromitWithStdin(testBinary, tmpDir, nil, "")
	if err != nil {
		t.Fatalf("RunGromitWithStdin() error = %v", err)
	}

	if exitCode != 0 {
		t.Fatalf("RunGromitWithStdin() exitCode = %d, want 0", exitCode)
	}

	// The output should be tmpDir
	gotDir := strings.TrimSpace(stdout)
	if gotDir != tmpDir {
		t.Errorf("With dir=%q, command ran in %q", tmpDir, gotDir)
	}
}

func TestRunGromitHelperProcessWithStdin(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	scriptContent := `#!/bin/bash
if [[ "$1" == "-test.run=TestGromitHelperProcess" && "$2" == "--" ]]; then
  shift 2
fi
echo "helper:$*"
cat
`
	testBinary := createTempShellScript(t, tmpDir, "helper-script", scriptContent)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stdout, stderr, exitCode, err := RunGromitHelperProcessWithStdin(ctx, testBinary, "", os.Environ(), "stdin payload\n", "debug", "test")
	if err != nil {
		t.Fatalf("RunGromitHelperProcessWithStdin() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout, "helper:debug test") {
		t.Fatalf("expected helper args in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, "stdin payload") {
		t.Fatalf("expected stdin in stdout, got %q", stdout)
	}
}

func TestRunGromitHelperProcessWithStdin_Timeout(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	scriptContent := `#!/bin/bash
sleep 5
`
	testBinary := createTempShellScript(t, tmpDir, "sleep-script", scriptContent)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, exitCode, err := RunGromitHelperProcessWithStdin(ctx, testBinary, "", os.Environ(), "")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "command canceled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	if exitCode != -1 {
		t.Fatalf("exitCode = %d, want -1 on timeout", exitCode)
	}
}

func TestRunGromitHelperProcessWithStdin_TimeoutKillsProcessGroup(t *testing.T) {
	tmpDir := t.TempDir()
	childPIDPath := filepath.Join(tmpDir, "child.pid")

	scriptContent := `#!/bin/bash
if [[ "$1" == "-test.run=TestGromitHelperProcess" && "$2" == "--" ]]; then
  shift 2
fi
child_pid_file="$1"
sleep 5 &
child=$!
echo "$child" > "$child_pid_file"
sleep 5
`
	testBinary := createTempShellScript(t, tmpDir, "sleep-with-child", scriptContent)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, _, _, err := RunGromitHelperProcessWithStdin(ctx, testBinary, "", os.Environ(), "", childPIDPath)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	childPIDBytes, readErr := os.ReadFile(childPIDPath)
	if readErr != nil {
		t.Fatalf("failed reading child pid file: %v", readErr)
	}
	childPID, parseErr := strconv.Atoi(strings.TrimSpace(string(childPIDBytes)))
	if parseErr != nil {
		t.Fatalf("failed parsing child pid: %v", parseErr)
	}

	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		err := syscall.Kill(childPID, 0)
		if err == nil {
			if time.Now().After(deadline) {
				t.Fatalf("expected child process to be terminated, pid=%d still running", childPID)
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if errors.Is(err, syscall.ESRCH) {
			break
		}
		t.Fatalf("unexpected error checking child process: %v", err)
	}
}

// TestCreateTempShellScript_ConcurrentCreationAndExecution verifies that
// createTempShellScript hardening prevents ETXTBSY races when multiple
// goroutines create and immediately execute scripts in parallel.
func TestCreateTempShellScript_ConcurrentCreationAndExecution(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scriptContent := `#!/bin/bash
echo "test output"
`

	const numConcurrent = 20
	errors := make([]error, numConcurrent)

	// Create a wait group to ensure all goroutines finish
	var wg sync.WaitGroup
	wg.Add(numConcurrent)

	// Launch multiple goroutines that each create and execute a script
	// with the same base name, simulating the parallel subtest scenario
	for i := 0; i < numConcurrent; i++ {
		go func(index int) {
			defer wg.Done()
			// All use the same base name "test-script" to stress the hardening
			scriptPath := createTempShellScript(t, tmpDir, "test-script", scriptContent)
			stdout, _, exitCode, err := RunGromitWithStdin(scriptPath, "", nil, "")
			if err != nil {
				errors[index] = fmt.Errorf("execution %d failed: %w", index, err)
				return
			}
			if exitCode != 0 {
				errors[index] = fmt.Errorf("execution %d exited with code %d", index, exitCode)
				return
			}
			if !strings.Contains(stdout, "test output") {
				errors[index] = fmt.Errorf("execution %d missing expected output", index)
				return
			}
		}(i)
	}

	wg.Wait()

	// Check for any ETXTBSY or other errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("error[%d]: %v", i, err)
		}
	}
}
