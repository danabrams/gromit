//go:build contract

package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexHarnessContractBuildTagCoverage(t *testing.T) {
	path := filepath.Join("codex_harness_test.go")
	if !fileHasBuildTag(t, path, "contract") {
		t.Fatalf("%s must include //go:build contract", path)
	}
}

func TestCodexHarnessContractAcceptanceArtifactRemoved(t *testing.T) {
	path := filepath.Join("codex_harness_acceptance_test.go")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s should not exist; contract coverage is in codex_harness_test.go", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func fileHasBuildTag(t *testing.T, path string, tag string) bool {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	lines := strings.Split(string(src), "\n")
	for i := 0; i < len(lines) && i < 10; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "//go:build "+tag || strings.HasPrefix(line, "//go:build "+tag+" ") {
			return true
		}
		if line == "// +build "+tag || strings.HasPrefix(line, "// +build "+tag+" ") {
			return true
		}
		if strings.HasPrefix(line, "package ") {
			return false
		}
	}

	return false
}
