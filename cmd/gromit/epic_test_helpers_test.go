package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type epicStatusFixtureResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

var (
	complexEpicStatusOnce   sync.Once
	complexEpicStatusResult epicStatusFixtureResult
)

func prependFakeTools(t *testing.T, testDir string) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	fakesDir := filepath.Join(wd, "..", "..", "test", "fakes")
	path := os.Getenv("PATH")
	t.Setenv("PATH", fakesDir+string(os.PathListSeparator)+path)
	t.Setenv("TEST_DIR", testDir)
}

// chdirTo changes to dir and restores the original working directory on test cleanup.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	t.Chdir(dir)
}

// writeGromitConfig writes a minimal gromit.yaml to tmpDir configuring the provided path directories.
// Pass specsDir and/or plansDir as empty string to omit them from the config.
func writeGromitConfig(t *testing.T, tmpDir, epicsDir, specsDir, plansDir string) {
	t.Helper()
	config := fmt.Sprintf("paths:\n  gromit_dir: %s\n  epics_dir: %s\n", filepath.Join(tmpDir, ".gromit"), epicsDir)
	if specsDir != "" {
		config += fmt.Sprintf("  specs_dir: %s\n", specsDir)
	}
	if plansDir != "" {
		config += fmt.Sprintf("  plans_dir: %s\n", plansDir)
	}
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("failed to write gromit.yaml: %v", err)
	}
}

// runComplexEpicStatusFixture builds a representative epic/spec/plan workspace once and
// reuses the single `epic status` invocation output across table/pipeline/linkage assertions.
func runComplexEpicStatusFixture(t *testing.T) (stdout, stderr string, exitCode int) {
	t.Helper()

	complexEpicStatusOnce.Do(func() {
		tmpDir := t.TempDir()
		epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
		specsDir := filepath.Join(tmpDir, ".gromit", "specs")
		plansDir := filepath.Join(tmpDir, ".gromit", "plans")

		for _, dir := range []string{epicsDir, specsDir, plansDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				complexEpicStatusResult.err = fmt.Errorf("failed to create directory %s: %w", dir, err)
				return
			}
		}

		epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# Developer Onboarding
`
		if err := os.WriteFile(filepath.Join(epicsDir, "onboarding.md"), []byte(epicContent), 0644); err != nil {
			complexEpicStatusResult.err = fmt.Errorf("failed to write epic: %w", err)
			return
		}

		specs := []struct {
			id      string
			title   string
			hasPlan bool
			decomp  bool
		}{
			{id: "auth-spec", title: "User Authentication", hasPlan: true, decomp: true},
			{id: "profile-spec", title: "User Profile", hasPlan: true, decomp: false},
			{id: "docs-spec", title: "Documentation", hasPlan: false, decomp: false},
		}

		for _, spec := range specs {
			specContent := fmt.Sprintf(`---
id: %s
epic: gromit-xyz
created: 2026-02-08
---

# %s
`, spec.id, spec.title)
			if err := os.WriteFile(filepath.Join(specsDir, spec.id+".md"), []byte(specContent), 0644); err != nil {
				complexEpicStatusResult.err = fmt.Errorf("failed to write spec %s: %w", spec.id, err)
				return
			}
			if !spec.hasPlan {
				continue
			}
			planContent := fmt.Sprintf(`---
id: %s
source_spec: %s
created: 2026-02-08
decomposed: %v
---

# Plan
`, spec.id, spec.id, spec.decomp)
			if err := os.WriteFile(filepath.Join(plansDir, spec.id+".md"), []byte(planContent), 0644); err != nil {
				complexEpicStatusResult.err = fmt.Errorf("failed to write plan for %s: %w", spec.id, err)
				return
			}
		}

		writeGromitConfig(t, tmpDir, epicsDir, specsDir, plansDir)

		t.Chdir(tmpDir)

		complexEpicStatusResult.stdout, complexEpicStatusResult.stderr, complexEpicStatusResult.exitCode = runGromitCobra(t, "epic", "status", "gromit-xyz")
	})

	if complexEpicStatusResult.err != nil {
		t.Fatalf("failed to prepare epic status fixture: %v", complexEpicStatusResult.err)
	}
	return complexEpicStatusResult.stdout, complexEpicStatusResult.stderr, complexEpicStatusResult.exitCode
}
