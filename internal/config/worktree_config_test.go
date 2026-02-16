package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeConfigUnmarshal(t *testing.T) {
	yamlContent := `
models:
  p0: opus
  p1: sonnet
worktree:
  enabled: true
  auto_merge: false
  merge_failure: "stop"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Worktree.Enabled == nil || !*cfg.Worktree.Enabled {
		t.Errorf("Worktree.Enabled = %v, want true", cfg.Worktree.Enabled)
	}
	if cfg.Worktree.AutoMerge == nil || *cfg.Worktree.AutoMerge {
		t.Errorf("Worktree.AutoMerge = %v, want false", cfg.Worktree.AutoMerge)
	}
	if cfg.Worktree.MergeFailure != "stop" {
		t.Errorf("Worktree.MergeFailure = %q, want %q", cfg.Worktree.MergeFailure, "stop")
	}
}

func TestWorktreeConfigUnmarshal_ConflictResolutionAndRetryCap(t *testing.T) {
	// Expected failure: WorktreeConfig lacks ConflictResolution/RetryCap fields
	// so parsing these settings is not supported yet.

	yamlContent := `
models:
  p0: opus
  p1: sonnet
worktree:
  enabled: true
  auto_merge: false
  merge_failure: "stop"
  conflict_resolution: "abort"
  retry_cap: 4
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Worktree.ConflictResolution != "abort" {
		t.Errorf("Worktree.ConflictResolution = %q, want %q", cfg.Worktree.ConflictResolution, "abort")
	}
	if cfg.Worktree.RetryCap != 4 {
		t.Errorf("Worktree.RetryCap = %d, want %d", cfg.Worktree.RetryCap, 4)
	}
}

func TestWorktreeIsEnabledNilPointer(t *testing.T) {
	cfg := WorktreeConfig{}
	if !cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return true for nil pointer (default enabled)")
	}
}

func TestWorktreeIsEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	cfg := WorktreeConfig{Enabled: &trueVal}
	if !cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return true for explicit true")
	}
}

func TestWorktreeIsEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	cfg := WorktreeConfig{Enabled: &falseVal}
	if cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return false for explicit false")
	}
}

func TestWorktreeIsAutoMergeEnabledNilPointer(t *testing.T) {
	cfg := WorktreeConfig{}
	if !cfg.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return true for nil pointer (default enabled)")
	}
}

func TestWorktreeIsAutoMergeEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	cfg := WorktreeConfig{AutoMerge: &trueVal}
	if !cfg.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return true for explicit true")
	}
}

func TestWorktreeIsAutoMergeEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	cfg := WorktreeConfig{AutoMerge: &falseVal}
	if cfg.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return false for explicit false")
	}
}

func TestWorktreeConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Defaults should be: Enabled=true, AutoMerge=true, MergeFailure="warn"
	if cfg.Worktree.Enabled == nil {
		t.Errorf("expected Worktree.Enabled to have default value (true), got nil")
	} else if !*cfg.Worktree.Enabled {
		t.Errorf("expected Worktree.Enabled default to be true, got %v", *cfg.Worktree.Enabled)
	}

	if cfg.Worktree.AutoMerge == nil {
		t.Errorf("expected Worktree.AutoMerge to have default value (true), got nil")
	} else if !*cfg.Worktree.AutoMerge {
		t.Errorf("expected Worktree.AutoMerge default to be true, got %v", *cfg.Worktree.AutoMerge)
	}

	if cfg.Worktree.MergeFailure != "warn" {
		t.Errorf("expected Worktree.MergeFailure default to be %q, got %q", "warn", cfg.Worktree.MergeFailure)
	}
}

func TestWorktreeConfigDefaults_ConflictResolutionAndRetryCap(t *testing.T) {
	// Expected failure: WorktreeConfig defaults do not include conflict resolution or retry cap yet.

	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.Worktree.ConflictResolution != "abort" {
		t.Errorf("expected Worktree.ConflictResolution default to be %q, got %q", "abort", cfg.Worktree.ConflictResolution)
	}
	if cfg.Worktree.RetryCap != 3 {
		t.Errorf("expected Worktree.RetryCap default to be %d, got %d", 3, cfg.Worktree.RetryCap)
	}
}

func TestWorktreeConfigDefaultsNotOverriddenWhenSet(t *testing.T) {
	falseVal := false
	cfg := &Config{
		Worktree: WorktreeConfig{
			Enabled:      &falseVal,
			AutoMerge:    &falseVal,
			MergeFailure: "stop",
		},
	}
	cfg.SetDefaults()

	// Verify SetDefaults doesn't override explicit values
	if cfg.Worktree.Enabled == nil || *cfg.Worktree.Enabled {
		t.Errorf("expected Worktree.Enabled to remain false, got %v", cfg.Worktree.Enabled)
	}
	if cfg.Worktree.AutoMerge == nil || *cfg.Worktree.AutoMerge {
		t.Errorf("expected Worktree.AutoMerge to remain false, got %v", cfg.Worktree.AutoMerge)
	}
	if cfg.Worktree.MergeFailure != "stop" {
		t.Errorf("expected Worktree.MergeFailure to remain %q, got %q", "stop", cfg.Worktree.MergeFailure)
	}
}

func TestWorktreeConfigInFullConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
worktree:
  enabled: false
  auto_merge: true
  merge_failure: "stop"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Worktree.Enabled == nil {
		t.Fatal("expected Worktree.Enabled to be non-nil")
	}
	if *cfg.Worktree.Enabled {
		t.Errorf("expected Worktree.Enabled to be false, got %v", *cfg.Worktree.Enabled)
	}

	if cfg.Worktree.AutoMerge == nil {
		t.Fatal("expected Worktree.AutoMerge to be non-nil")
	}
	if !*cfg.Worktree.AutoMerge {
		t.Errorf("expected Worktree.AutoMerge to be true, got %v", *cfg.Worktree.AutoMerge)
	}

	if cfg.Worktree.MergeFailure != "stop" {
		t.Errorf("expected Worktree.MergeFailure to be %q, got %q", "stop", cfg.Worktree.MergeFailure)
	}

	// Verify convenience methods work correctly
	if cfg.Worktree.IsEnabled() {
		t.Errorf("expected IsEnabled() to return false, got true")
	}

	if !cfg.Worktree.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return true, got false")
	}
}

func TestWorktreeConfigOmittedFromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// When worktree section is omitted, defaults should be applied
	if !cfg.Worktree.IsEnabled() {
		t.Errorf("expected Worktree.IsEnabled() to return true (default), got false")
	}

	if !cfg.Worktree.IsAutoMergeEnabled() {
		t.Errorf("expected Worktree.IsAutoMergeEnabled() to return true (default), got false")
	}

	if cfg.Worktree.MergeFailure != "warn" {
		t.Errorf("expected Worktree.MergeFailure to be %q (default), got %q", "warn", cfg.Worktree.MergeFailure)
	}
}
