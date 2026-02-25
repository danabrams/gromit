package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	repoConfigName = "gromit.yaml"
	repoDirName    = ".gromit"
)

func ensureRepoRoot() error {
	root, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	rootAbs, err := absPath(root, "project root")
	if err != nil {
		return err
	}
	cwdAbs, err := absPath(cwd, "working directory")
	if err != nil {
		return err
	}

	if rootAbs == cwdAbs {
		return nil
	}
	if err := os.Chdir(rootAbs); err != nil {
		return fmt.Errorf("change to project root: %w", err)
	}
	return nil
}

// findProjectRoot walks up from the current directory to find the project root
// (identified by the presence of gromit.yaml or .gromit/ directory).
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	nearestGromitDir := ""
	for {
		if hasRepoMarker(dir, repoConfigName) {
			return dir, nil
		}
		if hasRepoMarker(dir, repoDirName) {
			if nearestGromitDir == "" {
				nearestGromitDir = dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			if nearestGromitDir != "" {
				return nearestGromitDir, nil
			}
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

func absPath(path, label string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	return abs, nil
}

func hasRepoMarker(dir, marker string) bool {
	_, err := os.Stat(filepath.Join(dir, marker))
	return err == nil
}

// resolveProjectPath resolves a path relative to the project root, using the caller's
// file location to determine the project root instead of relying on the current working directory.
// This allows tests to work correctly when run from any directory.
func resolveProjectPath(caller string, relativePath string) string {
	root := getProjectRootFromTestFile(caller)
	if root == "" {
		return relativePath
	}
	return filepath.Join(root, relativePath)
}

// getProjectRootFromTestFile finds the project root starting from the caller's file location.
// This function uses runtime.Caller to get the test file's directory, then walks up
// to find the project root. This works correctly regardless of the current working directory.
func getProjectRootFromTestFile(caller string) string {
	// Get the caller's file location using runtime.Caller
	// We skip 2 frames: runtime.Caller itself and this function
	// to get back to the caller of getProjectRootFromTestFile (which is resolveProjectPath)
	// But we actually need to skip 3 to get the actual caller of resolveProjectPath
	_, callerFile, _, ok := runtime.Caller(2)
	if !ok {
		// Fallback to using current working directory if we can't get caller info
		if root, err := findProjectRoot(); err == nil {
			return root
		}
		return ""
	}

	// Start from the directory containing the caller's file
	dir := filepath.Dir(callerFile)

	// Walk up to find the project root
	for {
		if hasRepoMarker(dir, repoConfigName) {
			return dir
		}
		if hasRepoMarker(dir, repoDirName) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding project root
			// Fallback to using findProjectRoot
			if root, err := findProjectRoot(); err == nil {
				return root
			}
			return ""
		}
		dir = parent
	}
}
