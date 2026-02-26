package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func currentDir(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}
	return dir
}

func assertCurrentDir(t *testing.T, want string) {
	t.Helper()

	if got := currentDir(t); got != want {
		t.Fatalf("working directory = %q, want %q", got, want)
	}
}

func TestRunInDir_NilCallback(t *testing.T) {
	t.Parallel()
	err := runInDir("", nil)
	if err == nil {
		t.Fatal("runInDir returned nil error for nil callback")
	}
	if err.Error() != "callback is nil" {
		t.Fatalf("error = %q, want %q", err.Error(), "callback is nil")
	}
}

func TestRunInDir_EmptyDirRunsInCurrentDirectory(t *testing.T) {
	// Not parallel: checks os.Getwd() which races with tests that call os.Chdir.
	originalDir := currentDir(t)

	called := false
	err := runInDir("", func() error {
		called = true
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if wd != originalDir {
			t.Fatalf("callback working directory = %q, want %q", wd, originalDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("runInDir returned error: %v", err)
	}
	if !called {
		t.Fatal("callback was not called")
	}
}

func TestRunInDir_NonexistentDirectory(t *testing.T) {
	// Not parallel: runInDir calls os.Chdir.
	originalDir := currentDir(t)

	nonexistent := filepath.Join(t.TempDir(), "does-not-exist")
	called := false
	err := runInDir(nonexistent, func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("runInDir returned nil error for nonexistent directory")
	}
	if called {
		t.Fatal("callback was called for nonexistent directory")
	}

	assertCurrentDir(t, originalDir)
}

func TestRunInDir_PropagatesCallbackError(t *testing.T) {
	// Not parallel: runInDir calls os.Chdir.
	targetDir := t.TempDir()
	wantErr := errors.New("callback failed")

	err := runInDir(targetDir, func() error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestRunInDir_RestoresOriginalDirectoryOnCallbackError(t *testing.T) {
	// Not parallel: runInDir calls os.Chdir.
	originalDir := currentDir(t)

	targetDir := t.TempDir()
	wantErr := errors.New("callback failed")

	err := runInDir(targetDir, func() error {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		if wd != targetDir {
			t.Fatalf("callback working directory = %q, want %q", wd, targetDir)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}

	assertCurrentDir(t, originalDir)
}
