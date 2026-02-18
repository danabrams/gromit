package testutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunGromitWithStdin(t *testing.T) {
	// Create a temporary test binary that echoes stdin
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "test-echo")

	// Write a simple script that echoes stdin and exits with the first arg as exit code
	scriptContent := `#!/bin/bash
cat
exit ${1:-0}
`
	if err := os.WriteFile(testBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	tests := []struct {
		name         string
		binary       string
		dir          string
		environ      []string
		stdin        string
		args         []string
		wantStdout   string
		wantExitCode int
		wantErr      bool
	}{
		{
			name:         "basic stdin echo",
			binary:       testBinary,
			dir:          "",
			environ:      nil,
			stdin:        "hello world\n",
			args:         []string{"0"},
			wantStdout:   "hello world\n",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "non-zero exit code",
			binary:       testBinary,
			dir:          "",
			environ:      nil,
			stdin:        "test input\n",
			args:         []string{"42"},
			wantStdout:   "test input\n",
			wantExitCode: 42,
			wantErr:      false,
		},
		{
			name:         "empty stdin",
			binary:       testBinary,
			dir:          "",
			environ:      nil,
			stdin:        "",
			args:         []string{"0"},
			wantStdout:   "",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "with dir set",
			binary:       testBinary,
			dir:          tmpDir,
			environ:      nil,
			stdin:        "test\n",
			args:         []string{"0"},
			wantStdout:   "test\n",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "with environ set",
			binary:       testBinary,
			dir:          "",
			environ:      []string{"FOO=bar"},
			stdin:        "test\n",
			args:         []string{"0"},
			wantStdout:   "test\n",
			wantExitCode: 0,
			wantErr:      false,
		},
		{
			name:         "nonexistent binary",
			binary:       "/nonexistent/binary",
			dir:          "",
			environ:      nil,
			stdin:        "test\n",
			args:         []string{},
			wantStdout:   "",
			wantExitCode: -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode, err := RunGromitWithStdin(tt.binary, tt.dir, tt.environ, tt.stdin, tt.args...)

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

func TestRunGromitWithStdin_EmptyDirNotSet(t *testing.T) {
	// This test verifies that when dir is empty, cmd.Dir is not set
	// We can't directly observe cmd.Dir after execution, but we can verify
	// the command runs in the current directory by checking the working directory

	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "pwd-script")

	// Write a script that prints the current working directory
	scriptContent := `#!/bin/bash
pwd
`
	if err := os.WriteFile(testBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

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
	// This test verifies that when dir is non-empty, cmd.Dir is set correctly

	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "pwd-script")

	// Write a script that prints the current working directory
	scriptContent := `#!/bin/bash
pwd
`
	if err := os.WriteFile(testBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

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
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "helper-script")

	scriptContent := `#!/bin/bash
if [[ "$1" == "-test.run=TestGromitHelperProcess" && "$2" == "--" ]]; then
  shift 2
fi
echo "helper:$*"
cat
`
	if err := os.WriteFile(testBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create helper script: %v", err)
	}

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
	tmpDir := t.TempDir()
	testBinary := filepath.Join(tmpDir, "sleep-script")

	scriptContent := `#!/bin/bash
sleep 5
`
	if err := os.WriteFile(testBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create sleep script: %v", err)
	}

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
	testBinary := filepath.Join(tmpDir, "sleep-with-child")
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
	if err := os.WriteFile(testBinary, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create child script: %v", err)
	}

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
