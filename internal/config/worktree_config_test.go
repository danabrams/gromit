package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Expected failure: WorktreeConfig type does not exist yet
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

	// Expected failure: WorktreeConfig struct and its fields do not exist
	if !cfg.Worktree.Enabled {
		t.Errorf("Worktree.Enabled = %v, want true", cfg.Worktree.Enabled)
	}
	if cfg.Worktree.AutoMerge {
		t.Errorf("Worktree.AutoMerge = %v, want false", cfg.Worktree.AutoMerge)
	}
	if cfg.Worktree.MergeFailure != "stop" {
		t.Errorf("Worktree.MergeFailure = %q, want %q", cfg.Worktree.MergeFailure, "stop")
	}
}

// Expected failure: WorktreeConfig type and IsEnabled() method do not exist
func TestWorktreeIsEnabledNilPointer(t *testing.T) {
	cfg := WorktreeConfig{}
	// Expected failure: IsEnabled() method does not exist on WorktreeConfig
	if !cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return true for nil pointer (default enabled)")
	}
}

// Expected failure: WorktreeConfig type and IsEnabled() method do not exist
func TestWorktreeIsEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	// Expected failure: WorktreeConfig struct and Enabled field do not exist
	cfg := WorktreeConfig{Enabled: &trueVal}
	// Expected failure: IsEnabled() method does not exist on WorktreeConfig
	if !cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return true for explicit true")
	}
}

// Expected failure: WorktreeConfig type and IsEnabled() method do not exist
func TestWorktreeIsEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	// Expected failure: WorktreeConfig struct and Enabled field do not exist
	cfg := WorktreeConfig{Enabled: &falseVal}
	// Expected failure: IsEnabled() method does not exist on WorktreeConfig
	if cfg.IsEnabled() {
		t.Errorf("expected IsEnabled() to return false for explicit false")
	}
}

// Expected failure: WorktreeConfig type and IsAutoMergeEnabled() method do not exist
func TestWorktreeIsAutoMergeEnabledNilPointer(t *testing.T) {
	cfg := WorktreeConfig{}
	// Expected failure: IsAutoMergeEnabled() method does not exist on WorktreeConfig
	if !cfg.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return true for nil pointer (default enabled)")
	}
}

// Expected failure: WorktreeConfig type and IsAutoMergeEnabled() method do not exist
func TestWorktreeIsAutoMergeEnabledExplicitTrue(t *testing.T) {
	trueVal := true
	// Expected failure: WorktreeConfig struct and AutoMerge field do not exist
	cfg := WorktreeConfig{AutoMerge: &trueVal}
	// Expected failure: IsAutoMergeEnabled() method does not exist on WorktreeConfig
	if !cfg.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return true for explicit true")
	}
}

// Expected failure: WorktreeConfig type and IsAutoMergeEnabled() method do not exist
func TestWorktreeIsAutoMergeEnabledExplicitFalse(t *testing.T) {
	falseVal := false
	// Expected failure: WorktreeConfig struct and AutoMerge field do not exist
	cfg := WorktreeConfig{AutoMerge: &falseVal}
	// Expected failure: IsAutoMergeEnabled() method does not exist on WorktreeConfig
	if cfg.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return false for explicit false")
	}
}

// Expected failure: Config.Worktree field does not exist
func TestWorktreeConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Expected failure: Config.Worktree field does not exist
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

// Expected failure: Config.Worktree field does not exist
func TestWorktreeConfigDefaultsNotOverriddenWhenSet(t *testing.T) {
	falseVal := false
	// Expected failure: Config.Worktree field does not exist
	cfg := &Config{
		Worktree: WorktreeConfig{
			Enabled:      &falseVal,
			AutoMerge:    &falseVal,
			MergeFailure: "stop",
		},
	}
	cfg.SetDefaults()

	// Verify SetDefaults doesn't override explicit values
	// Expected failure: Config.Worktree field does not exist
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

// Expected failure: Config.Worktree field does not exist
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

	// Expected failure: Config.Worktree field does not exist
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
	// Expected failure: IsEnabled() method does not exist on WorktreeConfig
	if cfg.Worktree.IsEnabled() {
		t.Errorf("expected IsEnabled() to return false, got true")
	}

	// Expected failure: IsAutoMergeEnabled() method does not exist on WorktreeConfig
	if !cfg.Worktree.IsAutoMergeEnabled() {
		t.Errorf("expected IsAutoMergeEnabled() to return true, got false")
	}
}

// Expected failure: Config.Worktree field does not exist
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
	// Expected failure: Config.Worktree field and convenience methods do not exist
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
