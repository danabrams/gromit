//go:build contract

package contracts

import (
	"os"
	"strings"
	"testing"
)

const (
	contractHarnessTestFile           = "codex_harness_test.go"
	contractAcceptanceArtifactFile    = "codex_harness_acceptance_test.go"
	buildTagHeaderScanLineLimit       = 10
	contractDefaultAcceptanceBuildTag = "acceptance"
)

func TestCodexHarnessContractBuildTagCoverage(t *testing.T) {
	path := contractHarnessTestFile
	if !fileHasBuildTag(t, path, "contract") {
		t.Fatalf("%s must include //go:build contract", path)
	}
}

func TestCodexHarnessContractAcceptanceArtifactRemoved(t *testing.T) {
	path := contractAcceptanceArtifactFile
	_, acceptanceErr := os.Stat(path)
	_, reclassifiedErr := os.Stat(contractHarnessTestFile)

	switch {
	case acceptanceErr == nil:
		if !fileHasBuildTag(t, path, contractDefaultAcceptanceBuildTag) {
			t.Fatalf("%s must include //go:build %s", path, contractDefaultAcceptanceBuildTag)
		}
	case reclassifiedErr == nil:
		// Coverage is intentionally reclassified into codex_harness_test.go under //go:build contract.
	case os.IsNotExist(acceptanceErr) && os.IsNotExist(reclassifiedErr):
		t.Fatalf("expected either %s or %s to exist", path, contractHarnessTestFile)
	default:
		if acceptanceErr != nil && !os.IsNotExist(acceptanceErr) {
			t.Fatalf("stat %s: %v", path, acceptanceErr)
		}
		if reclassifiedErr != nil && !os.IsNotExist(reclassifiedErr) {
			t.Fatalf("stat %s: %v", contractHarnessTestFile, reclassifiedErr)
		}
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
