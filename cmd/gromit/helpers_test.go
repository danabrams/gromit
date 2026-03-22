package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// getRepoRoot finds the repo root by walking up from the current test file until go.mod is found.
func getRepoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("could not get caller info")
	}

	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached filesystem root without finding go.mod
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}
