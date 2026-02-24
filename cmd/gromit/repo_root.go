package main

import (
	"fmt"
	"os"
	"path/filepath"
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
