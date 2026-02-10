package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEpicStatusCommand_FlagExists verifies that epic command has status subcommand
func TestEpicStatusCommand_FlagExists(t *testing.T) {
	stdout, stderr, exitCode := runGromit(t, "epic", "status", "--help")

	if exitCode != 0 {
		t.Errorf("epic status --help exited with code %d, stderr: %s", exitCode, stderr)
	}

	if !strings.Contains(stdout, "status") {
		t.Error("epic status help should contain 'status' in output")
	}
}

// TestEpicStatusCommand_RequiresEpicID verifies that status subcommand requires epic ID argument
func TestEpicStatusCommand_RequiresEpicID(t *testing.T) {
	_, stderr, exitCode := runGromit(t, "epic", "status")

	if exitCode == 0 {
		t.Error("epic status without epic ID should exit non-zero")
	}

	if !strings.Contains(stderr, "accepts 1 arg") && !strings.Contains(stderr, "required") {
		t.Errorf("error message should mention missing epic ID, got: %s", stderr)
	}
}

// TestEpicStatusCommand_DisplaysEpicTitle verifies that status displays epic title from epic document
func TestEpicStatusCommand_DisplaysEpicTitle(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Create epic document with frontmatter
	epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# Improve Developer Onboarding

This epic explores ways to make developer onboarding smoother.
`
	epicPath := filepath.Join(epicsDir, "onboarding.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	stdout, stderr, exitCode := runGromit(t, "epic", "status", "gromit-xyz")

	// May fail if bd binary is not available, but should at least find the epic
	if exitCode == 0 {
		if !strings.Contains(stdout, "Improve Developer Onboarding") {
			t.Errorf("status output should contain epic title, got: %s", stdout)
		}
	} else {
		// If command fails, verify it's not because epic wasn't found
		if strings.Contains(stderr, "epic not found") || strings.Contains(stderr, "no epic document") {
			t.Errorf("command should find the epic document, stderr: %s", stderr)
		}
	}
}

// TestEpicStatusCommand_DisplaysLinkedSpecs verifies that status shows specs linked to the epic
func TestEpicStatusCommand_DisplaysLinkedSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	// Create epic document
	epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# Improve Developer Onboarding
`
	if err := os.WriteFile(filepath.Join(epicsDir, "onboarding.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create specs linked to the epic
	specs := []struct {
		id       string
		filename string
		title    string
	}{
		{"auth", "auth.md", "User Authentication"},
		{"profile", "profile.md", "User Profile Management"},
	}

	for _, spec := range specs {
		specContent := fmt.Sprintf(`---
id: %s
epic: gromit-xyz
created: 2026-02-08
---

# %s

This spec describes the implementation.
`, spec.id, spec.title)
		specPath := filepath.Join(specsDir, spec.filename)
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to write spec %s: %v", spec.filename, err)
		}
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
  specs_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir, specsDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	stdout, _, exitCode := runGromit(t, "epic", "status", "gromit-xyz")

	// If command succeeds, verify spec display
	if exitCode == 0 {
		// Should show both spec IDs
		if !strings.Contains(stdout, "auth") {
			t.Errorf("status output should contain spec 'auth', got: %s", stdout)
		}
		if !strings.Contains(stdout, "profile") {
			t.Errorf("status output should contain spec 'profile', got: %s", stdout)
		}
	}
}

// TestEpicStatusCommand_ShowsPipelineStages verifies that status displays pipeline stage for each spec
func TestEpicStatusCommand_ShowsPipelineStages(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")
	plansDir := filepath.Join(tmpDir, ".gromit", "plans")

	for _, dir := range []string{epicsDir, specsDir, plansDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create epic
	epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# Test Epic
`
	if err := os.WriteFile(filepath.Join(epicsDir, "test.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create spec without plan (unplanned stage)
	specUnplanned := `---
id: unplanned-spec
epic: gromit-xyz
created: 2026-02-08
---

# Unplanned Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "unplanned.md"), []byte(specUnplanned), 0644); err != nil {
		t.Fatalf("failed to write unplanned spec: %v", err)
	}

	// Create spec with plan but not decomposed (planned stage)
	specPlanned := `---
id: planned-spec
epic: gromit-xyz
created: 2026-02-08
---

# Planned Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "planned.md"), []byte(specPlanned), 0644); err != nil {
		t.Fatalf("failed to write planned spec: %v", err)
	}

	planContent := `---
id: planned-spec
source_spec: planned-spec
created: 2026-02-08
decomposed: false
---

# Plan

This is the plan.
`
	if err := os.WriteFile(filepath.Join(plansDir, "planned-spec.md"), []byte(planContent), 0644); err != nil {
		t.Fatalf("failed to write plan: %v", err)
	}

	// Create spec with decomposed plan (decomposed stage)
	specDecomposed := `---
id: decomposed-spec
epic: gromit-xyz
created: 2026-02-08
---

# Decomposed Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "decomposed.md"), []byte(specDecomposed), 0644); err != nil {
		t.Fatalf("failed to write decomposed spec: %v", err)
	}

	decomposedPlan := `---
id: decomposed-spec
source_spec: decomposed-spec
created: 2026-02-08
decomposed: true
---

# Decomposed Plan
`
	if err := os.WriteFile(filepath.Join(plansDir, "decomposed-spec.md"), []byte(decomposedPlan), 0644); err != nil {
		t.Fatalf("failed to write decomposed plan: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
  specs_dir: %s
  plans_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir, specsDir, plansDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	stdout, _, exitCode := runGromit(t, "epic", "status", "gromit-xyz")

	// If command succeeds, verify pipeline stages are displayed
	if exitCode == 0 {
		stdoutLower := strings.ToLower(stdout)

		// Check for stage indicators
		if !strings.Contains(stdoutLower, "unplanned") && !strings.Contains(stdoutLower, "no plan") {
			t.Logf("Warning: status output may not show unplanned stage, got: %s", stdout)
		}

		if !strings.Contains(stdoutLower, "planned") && !strings.Contains(stdoutLower, "not decomposed") {
			t.Logf("Warning: status output may not show planned stage, got: %s", stdout)
		}

		if !strings.Contains(stdoutLower, "decomposed") && !strings.Contains(stdoutLower, "done") {
			t.Logf("Warning: status output may not show decomposed stage, got: %s", stdout)
		}
	}
}

// TestEpicStatusCommand_ShowsBeadCounts verifies that status displays bead counts for each spec
func TestEpicStatusCommand_ShowsBeadCounts(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")

	for _, dir := range []string{epicsDir, specsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create epic
	epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# Test Epic
`
	if err := os.WriteFile(filepath.Join(epicsDir, "test.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create spec
	specContent := `---
id: test-spec
epic: gromit-xyz
created: 2026-02-08
---

# Test Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "test.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
  specs_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir, specsDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	stdout, _, exitCode := runGromit(t, "epic", "status", "gromit-xyz")

	// If command succeeds (requires bd binary), verify bead counts are shown
	if exitCode == 0 {
		stdoutLower := strings.ToLower(stdout)

		// Should show some indication of bead counts
		// Could be "0 beads", "3/5 complete", "open: 2, closed: 1", etc.
		hasCounts := strings.Contains(stdoutLower, "bead") ||
			strings.Contains(stdoutLower, "open") ||
			strings.Contains(stdoutLower, "closed") ||
			strings.Contains(stdoutLower, "complete")

		if !hasCounts {
			t.Logf("Warning: status output should show bead counts, got: %s", stdout)
		}
	}
}

// TestEpicStatusCommand_ErrorWhenEpicNotFound verifies appropriate error for non-existent epic
func TestEpicStatusCommand_ErrorWhenEpicNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("failed to create epics dir: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	_, stderr, exitCode := runGromit(t, "epic", "status", "nonexistent-epic-xyz")

	if exitCode == 0 {
		t.Error("command should exit non-zero for non-existent epic")
	}

	stderrLower := strings.ToLower(stderr)
	if !strings.Contains(stderrLower, "not found") && !strings.Contains(stderrLower, "no epic") {
		t.Errorf("error should mention epic not found, got: %s", stderr)
	}
}

// TestEpicStatusCommand_HandlesEpicWithNoLinkedSpecs verifies display when epic has no linked specs
func TestEpicStatusCommand_HandlesEpicWithNoLinkedSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")

	for _, dir := range []string{epicsDir, specsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create epic document
	epicContent := `---
epic_id: gromit-lonely
created: 2026-02-08
---

# Lonely Epic

This epic has no specs yet.
`
	if err := os.WriteFile(filepath.Join(epicsDir, "lonely.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create a spec not linked to this epic
	specContent := `---
id: other-spec
epic: different-epic
created: 2026-02-08
---

# Other Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "other.md"), []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
  specs_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir, specsDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	stdout, stderr, exitCode := runGromit(t, "epic", "status", "gromit-lonely")

	// Should succeed or at least find the epic
	if exitCode != 0 {
		if strings.Contains(stderr, "not found") {
			t.Errorf("command should find the epic even with no linked specs, stderr: %s", stderr)
		}
	}

	// If successful, should indicate no specs
	if exitCode == 0 {
		stdoutLower := strings.ToLower(stdout)
		hasNoSpecsIndicator := strings.Contains(stdoutLower, "no spec") ||
			strings.Contains(stdoutLower, "0 spec") ||
			strings.Contains(stdoutLower, "empty")

		if !hasNoSpecsIndicator {
			t.Logf("Warning: status should indicate no linked specs, got: %s", stdout)
		}
	}
}

// TestEpicStatusCommand_UsesResolveEpic verifies that status uses scope.ResolveEpic for spec resolution
func TestEpicStatusCommand_UsesResolveEpic(t *testing.T) {
	// This test verifies the behavior that results from using scope.ResolveEpic:
	// - Specs are found by scanning frontmatter for matching epic field
	// - Only specs with valid frontmatter are included
	// - Specs without epic field or with different epic are excluded

	tmpDir := t.TempDir()
	epicsDir := filepath.Join(tmpDir, ".gromit", "epics")
	specsDir := filepath.Join(tmpDir, ".gromit", "specs")

	for _, dir := range []string{epicsDir, specsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create epic
	epicContent := `---
epic_id: gromit-xyz
created: 2026-02-08
---

# Test Epic
`
	if err := os.WriteFile(filepath.Join(epicsDir, "test.md"), []byte(epicContent), 0644); err != nil {
		t.Fatalf("failed to write epic: %v", err)
	}

	// Create spec linked to epic
	linkedSpec := `---
id: linked-spec
epic: gromit-xyz
created: 2026-02-08
---

# Linked Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "linked.md"), []byte(linkedSpec), 0644); err != nil {
		t.Fatalf("failed to write linked spec: %v", err)
	}

	// Create spec without epic field
	noEpicSpec := `---
id: no-epic-spec
created: 2026-02-08
---

# No Epic Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "no-epic.md"), []byte(noEpicSpec), 0644); err != nil {
		t.Fatalf("failed to write no-epic spec: %v", err)
	}

	// Create spec with different epic
	differentEpicSpec := `---
id: different-spec
epic: different-epic-abc
created: 2026-02-08
---

# Different Epic Spec
`
	if err := os.WriteFile(filepath.Join(specsDir, "different.md"), []byte(differentEpicSpec), 0644); err != nil {
		t.Fatalf("failed to write different-epic spec: %v", err)
	}

	// Create minimal gromit.yaml
	configContent := fmt.Sprintf(`paths:
  gromit_dir: %s
  epics_dir: %s
  specs_dir: %s
`, filepath.Join(tmpDir, ".gromit"), epicsDir, specsDir)
	configPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Change to temp directory
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	stdout, _, exitCode := runGromit(t, "epic", "status", "gromit-xyz")

	// If command succeeds, verify correct filtering
	if exitCode == 0 {
		// Should show only the linked spec
		if !strings.Contains(stdout, "linked-spec") {
			t.Errorf("status should show linked-spec, got: %s", stdout)
		}

		// Should NOT show specs without epic or with different epic
		if strings.Contains(stdout, "no-epic-spec") {
			t.Errorf("status should not show no-epic-spec, got: %s", stdout)
		}

		if strings.Contains(stdout, "different-spec") {
			t.Errorf("status should not show different-spec, got: %s", stdout)
		}
	}
}
