package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
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
	for {
		if _, err := os.Stat(filepath.Join(dir, "gromit.yaml")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".gromit")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
