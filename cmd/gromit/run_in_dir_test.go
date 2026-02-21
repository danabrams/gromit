package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunInDir_NilCallback(t *testing.T) {
	err := runInDir("", nil)
	if err == nil {
		t.Fatal("runInDir returned nil error for nil callback")
	}
	if err.Error() != "callback is nil" {
		t.Fatalf("error = %q, want %q", err.Error(), "callback is nil")
	}
}

func TestRunInDir_EmptyDirRunsInCurrentDirectory(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}

	called := false
	err = runInDir("", func() error {
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
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}

	nonexistent := filepath.Join(t.TempDir(), "does-not-exist")
	called := false
	err = runInDir(nonexistent, func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("runInDir returned nil error for nonexistent directory")
	}
	if called {
		t.Fatal("callback was called for nonexistent directory")
	}

	currentDir, getErr := os.Getwd()
	if getErr != nil {
		t.Fatalf("Getwd() after runInDir failed: %v", getErr)
	}
	if currentDir != originalDir {
		t.Fatalf("working directory after runInDir = %q, want %q", currentDir, originalDir)
	}
}

func TestRunInDir_PropagatesCallbackError(t *testing.T) {
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
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() failed: %v", err)
	}

	targetDir := t.TempDir()
	wantErr := errors.New("callback failed")

	err = runInDir(targetDir, func() error {
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

	currentDir, getErr := os.Getwd()
	if getErr != nil {
		t.Fatalf("Getwd() after runInDir failed: %v", getErr)
	}
	if currentDir != originalDir {
		t.Fatalf("working directory after runInDir = %q, want %q", currentDir, originalDir)
	}
}
