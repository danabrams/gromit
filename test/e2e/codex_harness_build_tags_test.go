//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
)

const (
	e2eHarnessTestFile          = "codex_harness_test.go"
	e2eAcceptanceArtifactFile   = "codex_harness_acceptance_test.go"
	buildTagHeaderScanLineLimit = 10
)

func TestCodexHarnessE2EBuildTagCoverage(t *testing.T) {
	path := e2eHarnessTestFile
	if !fileHasBuildTag(t, path, "e2e") {
		t.Fatalf("%s must include //go:build e2e", path)
	}
}

func TestCodexHarnessE2EAcceptanceArtifactRemoved(t *testing.T) {
	path := e2eAcceptanceArtifactFile
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s should not exist; e2e coverage is in codex_harness_test.go", path)
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
	for i := 0; i < len(lines) && i < buildTagHeaderScanLineLimit; i++ {
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
