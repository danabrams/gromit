//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexHarnessAcceptanceFilesTaggedOrReclassified(t *testing.T) {
	// Expected failure: AssertCodexHarnessAcceptanceBuildTagsApplied does not exist yet, and codex harness
	// acceptance files still need build tags or reclassification.
	AssertCodexHarnessAcceptanceBuildTagsApplied(t)

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Fatalf("could not find project root: %v", err)
	}

	candidates := []string{
		filepath.Join("test", "contracts", "codex_harness_acceptance_test.go"),
		filepath.Join("test", "e2e", "codex_harness_acceptance_test.go"),
	}

	for _, rel := range candidates {
		full := filepath.Join(projectRoot, rel)
		info, err := os.Stat(full)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("stat %s: %v", rel, err)
		}
		if info.IsDir() {
			t.Fatalf("expected %s to be a file, found directory", rel)
		}
		if !hasAcceptedBuildTag(t, full) {
			t.Errorf("%s must include //go:build acceptance or //go:build e2e_live when named *_acceptance_test.go", rel)
		}
	}
}
