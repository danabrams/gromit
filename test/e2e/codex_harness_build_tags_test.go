//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
)

const (
	e2eHarnessTestFile           = "codex_harness_test.go"
	e2eAcceptanceArtifactFile    = "codex_harness_acceptance_test.go"
	buildTagHeaderScanLineLimit  = 10
	e2eDefaultAcceptanceBuildTag = "acceptance"
)

func TestCodexHarnessE2EBuildTagCoverage(t *testing.T) {
	path := e2eHarnessTestFile
	if !fileHasBuildTag(t, path, "e2e") {
		t.Fatalf("%s must include //go:build e2e", path)
	}
}

func TestCodexHarnessE2EAcceptanceArtifactRemoved(t *testing.T) {
	path := e2eAcceptanceArtifactFile
	_, acceptanceErr := os.Stat(path)
	_, reclassifiedErr := os.Stat(e2eHarnessTestFile)

	switch {
	case acceptanceErr == nil:
		if !fileHasBuildTag(t, path, e2eDefaultAcceptanceBuildTag) {
			t.Fatalf("%s must include //go:build %s", path, e2eDefaultAcceptanceBuildTag)
		}
	case reclassifiedErr == nil:
		// Coverage is intentionally reclassified into codex_harness_test.go under //go:build e2e.
	case os.IsNotExist(acceptanceErr) && os.IsNotExist(reclassifiedErr):
		t.Fatalf("expected either %s or %s to exist", path, e2eHarnessTestFile)
	default:
		if acceptanceErr != nil && !os.IsNotExist(acceptanceErr) {
			t.Fatalf("stat %s: %v", path, acceptanceErr)
		}
		if reclassifiedErr != nil && !os.IsNotExist(reclassifiedErr) {
			t.Fatalf("stat %s: %v", e2eHarnessTestFile, reclassifiedErr)
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
