//go:build contract

package contracts

import (
	"os"
	"strings"
	"testing"
)

const (
	contractHarnessTestFile        = "codex_harness_test.go"
	contractAcceptanceArtifactFile = "codex_harness_acceptance_test.go"
	buildTagHeaderScanLineLimit    = 10
)

func TestCodexHarnessContractBuildTagCoverage(t *testing.T) {
	path := contractHarnessTestFile
	if !fileHasBuildTag(t, path, "contract") {
		t.Fatalf("%s must include //go:build contract", path)
	}
}

func TestCodexHarnessContractAcceptanceArtifactRemoved(t *testing.T) {
	path := contractAcceptanceArtifactFile
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
