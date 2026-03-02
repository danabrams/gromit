package main

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const cmdGromitGofmtChunkSize = 200

func TestCmdGromitTestFilesGofmt(t *testing.T) {
	files := cmdGromitTestFiles(t)
	if len(files) == 0 {
		t.Fatalf("no cmd/gromit test files found")
	}

	if nonCompliant := gofmtNonCompliantFiles(t, files); len(nonCompliant) > 0 {
		t.Fatalf("gofmt -l reported non-compliant files:\n%s", strings.Join(nonCompliant, "\n"))
	}
}

func TestCmdGromitSourceFilesGofmt(t *testing.T) {
	files := cmdGromitSourceFiles(t)
	if len(files) == 0 {
		t.Fatalf("no cmd/gromit source files found")
	}

	if nonCompliant := gofmtNonCompliantFiles(t, files); len(nonCompliant) > 0 {
		t.Fatalf("gofmt -l reported non-compliant files:\n%s", strings.Join(nonCompliant, "\n"))
	}
}

func cmdGromitTestFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	if err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		files = append(files, abs)
		return nil
	}); err != nil {
		t.Fatalf("walking cmd/gromit tree: %v", err)
	}

	sort.Strings(files)
	return files
}

func gofmtNonCompliantFiles(t *testing.T, files []string) []string {
	t.Helper()

	var nonCompliant []string
	for start := 0; start < len(files); start += cmdGromitGofmtChunkSize {
		end := start + cmdGromitGofmtChunkSize
		if end > len(files) {
			end = len(files)
		}

		args := append([]string{"-l"}, files[start:end]...)
		cmd := exec.Command("gofmt", args...)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("gofmt -l failed: %v", err)
		}

		cleaned := strings.TrimSpace(string(out))
		if cleaned == "" {
			continue
		}

		for _, line := range strings.Split(cleaned, "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				nonCompliant = append(nonCompliant, trimmed)
			}
		}
	}

	return nonCompliant
}
