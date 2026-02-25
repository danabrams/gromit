package main

import (
	"path/filepath"
	"runtime"
	"testing"
)

func productionFilePath(t *testing.T, rel string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine caller file path")
	}

	root := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	return filepath.Join(root, rel)
}
