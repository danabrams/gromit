package main

import (
	"os"
	"path/filepath"
	"testing"
)

func projectRootPath(t testing.TB) string {
	t.Helper()
	if root := getProjectRootFromCaller(3); root != "" {
		return root
	}
	root, err := findProjectRoot()
	if err != nil {
		t.Fatalf("findProjectRoot: %v", err)
	}
	return root
}

func projectFilePath(t testing.TB, rel string) string {
	t.Helper()
	return filepath.Join(projectRootPath(t), rel)
}

func mustReadProjectFile(t testing.TB, rel string) string {
	t.Helper()
	path := projectFilePath(t, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}
