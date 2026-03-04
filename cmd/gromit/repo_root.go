package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	repoConfigName = "gromit.yaml"
	repoDirName    = ".gromit"
)

var initialWorkingDir string

func init() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}
	initialWorkingDir = wd
}

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
	if projectPath != "" {
		pathValue := projectPath
		if !filepath.IsAbs(pathValue) && initialWorkingDir != "" {
			pathValue = filepath.Join(initialWorkingDir, pathValue)
		}
		root, err := resolveProjectPathCandidate(pathValue)
		if err == nil {
			return root, nil
		}
		if !filepath.IsAbs(projectPath) && initialWorkingDir != "" && errors.Is(err, os.ErrNotExist) {
			if cwd, cwdErr := os.Getwd(); cwdErr == nil {
				fallback := filepath.Join(cwd, projectPath)
				if filepath.Clean(fallback) != filepath.Clean(pathValue) {
					return resolveProjectPathCandidate(fallback)
				}
			}
		}
		return "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		if root := getProjectRootFromCaller(3); root != "" {
			return root, nil
		}
		return "", err
	}

	if root := findRepoRootFromDir(cwd); root != "" {
		return root, nil
	}

	if root := getProjectRootFromCaller(3); root != "" {
		return root, nil
	}

	return "", os.ErrNotExist
}

func resolveProjectPathCandidate(path string) (string, error) {
	absPathValue, err := absPath(path, "project path flag")
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absPathValue)
	if err != nil {
		return "", fmt.Errorf("project path %q: %w", absPathValue, err)
	}
	dir := absPathValue
	if !info.IsDir() {
		dir = filepath.Dir(absPathValue)
	}
	if root := findRepoRootFromDir(dir); root != "" {
		return root, nil
	}
	return "", fmt.Errorf("project path %q does not contain %s or %s: %w", absPathValue, repoConfigName, repoDirName, os.ErrNotExist)
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
	if root := getProjectRootFromCaller(3); root != "" {
		return filepath.Join(root, relativePath)
	}
	if root, err := findProjectRoot(); err == nil {
		return filepath.Join(root, relativePath)
	}
	return relativePath
}

// findRepoRootFromDir walks up from dir looking for gromit.yaml or .gromit directories.
func findRepoRootFromDir(dir string) string {
	if dir == "" {
		return ""
	}

	nearest := ""
	for {
		if hasRepoMarker(dir, repoConfigName) {
			return dir
		}
		if hasRepoMarker(dir, repoDirName) && nearest == "" {
			nearest = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			if nearest != "" {
				return nearest
			}
			return ""
		}
		dir = parent
	}
}

// getProjectRootFromCaller resolves the project root using the caller's file location.
func getProjectRootFromCaller(skip int) string {
	for i := skip; i < skip+5; i++ {
		if root := findRepoRootFromCaller(i); root != "" {
			return root
		}
	}
	return ""
}

func findRepoRootFromCaller(skip int) string {
	_, callerFile, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	return findRepoRootFromDir(filepath.Dir(callerFile))
}

func getProjectRootFromTestFile(caller string) string {
	if root := getProjectRootFromCaller(3); root != "" {
		return root
	}
	return ""
}
