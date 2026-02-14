//go:build acceptance

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDecomposeConfigDefaults verifies that DecomposeConfig applies sensible defaults
func TestDecomposeConfigDefaults(t *testing.T) {
	// Expected failure: DecomposeConfig type does not exist yet
	cfg := &Config{}
	cfg.SetDefaults()

	// Expected failure: Config.Decompose field does not exist yet
	if !cfg.Decompose.IsValidateEnabled() {
		t.Errorf("expected default decompose validation enabled=true")
	}
	// Expected failure: DecomposeConfig.MaxValidationRetries field does not exist yet
	if cfg.Decompose.MaxValidationRetries != 2 {
		t.Errorf("expected default MaxValidationRetries=2, got %d", cfg.Decompose.MaxValidationRetries)
	}
}

// TestDecomposeConfigFromYAML verifies that decompose config can be loaded from YAML
func TestDecomposeConfigFromYAML(t *testing.T) {
	// Expected failure: DecomposeConfig type and YAML unmarshaling does not exist yet
	tests := []struct {
		name                     string
		yaml                     string
		expectValidateEnabled    bool
		expectMaxValidationRetry int
	}{
		{
			name: "All fields explicit",
			yaml: `decompose:
  validate: true
  max_validation_retries: 3
`,
			expectValidateEnabled:    true,
			expectMaxValidationRetry: 3,
		},
		{
			name: "Validation disabled explicitly",
			yaml: `decompose:
  validate: false
`,
			expectValidateEnabled:    false,
			expectMaxValidationRetry: 2, // default
		},
		{
			name: "Custom max retries only",
			yaml: `decompose:
  max_validation_retries: 1
`,
			expectValidateEnabled:    true, // default
			expectMaxValidationRetry: 1,
		},
		{
			name:                     "Empty config uses defaults",
			yaml:                     "",
			expectValidateEnabled:    true,
			expectMaxValidationRetry: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: DecomposeConfig does not exist and cannot be parsed from YAML
			dir := t.TempDir()
			cfgPath := filepath.Join(dir, "gromit.yaml")
			if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := Load(cfgPath)
			if err != nil {
				t.Fatalf("loading config: %v", err)
			}

			// Expected failure: Config.Decompose field and IsValidateEnabled method do not exist
			if cfg.Decompose.IsValidateEnabled() != tt.expectValidateEnabled {
				t.Errorf("expected validate enabled=%v, got %v", tt.expectValidateEnabled, cfg.Decompose.IsValidateEnabled())
			}
			// Expected failure: DecomposeConfig.MaxValidationRetries field does not exist
			if cfg.Decompose.MaxValidationRetries != tt.expectMaxValidationRetry {
				t.Errorf("expected max_validation_retries=%d, got %d", tt.expectMaxValidationRetry, cfg.Decompose.MaxValidationRetries)
			}
		})
	}
}

// TestDecomposeIsValidateEnabledNilPointer verifies IsValidateEnabled returns true when Validate is nil
func TestDecomposeIsValidateEnabledNilPointer(t *testing.T) {
	// Expected failure: DecomposeConfig type does not exist yet
	cfg := DecomposeConfig{}
	// Expected failure: IsValidateEnabled method does not exist yet
	if !cfg.IsValidateEnabled() {
		t.Errorf("expected IsValidateEnabled() to return true for nil pointer (default enabled)")
	}
}

// TestDecomposeIsValidateEnabledExplicitTrue verifies IsValidateEnabled returns true when explicitly set
func TestDecomposeIsValidateEnabledExplicitTrue(t *testing.T) {
	// Expected failure: DecomposeConfig type does not exist yet
	trueVal := true
	cfg := DecomposeConfig{Validate: &trueVal}
	// Expected failure: IsValidateEnabled method does not exist yet
	if !cfg.IsValidateEnabled() {
		t.Errorf("expected IsValidateEnabled() to return true for explicit true")
	}
}

// TestDecomposeIsValidateEnabledExplicitFalse verifies IsValidateEnabled returns false when explicitly disabled
func TestDecomposeIsValidateEnabledExplicitFalse(t *testing.T) {
	// Expected failure: DecomposeConfig type does not exist yet
	falseVal := false
	cfg := DecomposeConfig{Validate: &falseVal}
	// Expected failure: IsValidateEnabled method does not exist yet
	if cfg.IsValidateEnabled() {
		t.Errorf("expected IsValidateEnabled() to return false for explicit false")
	}
}

// TestDecomposeConfigInFullConfig verifies decompose config works alongside other config sections
func TestDecomposeConfigInFullConfig(t *testing.T) {
	// Expected failure: DecomposeConfig type and Config.Decompose field do not exist yet
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `models:
  p0: opus
  p1: sonnet
  p2: haiku
decompose:
  validate: false
  max_validation_retries: 5
validation:
  enabled: true
  commands:
    - "go test ./..."
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Expected failure: Config.Decompose field does not exist yet
	if cfg.Decompose.IsValidateEnabled() {
		t.Errorf("expected decompose validation disabled, got enabled")
	}
	// Expected failure: DecomposeConfig.MaxValidationRetries field does not exist yet
	if cfg.Decompose.MaxValidationRetries != 5 {
		t.Errorf("expected max_validation_retries=5, got %d", cfg.Decompose.MaxValidationRetries)
	}

	// Verify other config sections are unaffected
	if !cfg.Validation.Enabled {
		t.Errorf("expected validation.enabled=true, got false")
	}
	if cfg.Models.P0 != "opus" {
		t.Errorf("expected models.p0=opus, got %s", cfg.Models.P0)
	}
}

// TestDecomposeConfigZeroRetries verifies zero retries is preserved (not defaulted)
func TestDecomposeConfigZeroRetries(t *testing.T) {
	// Expected failure: DecomposeConfig type does not exist yet
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "gromit.yaml")
	yaml := `decompose:
  max_validation_retries: 0
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Expected failure: Config.Decompose field does not exist yet
	// Zero is the sentinel for 'not configured' — SetDefaults should NOT override explicit zero
	if cfg.Decompose.MaxValidationRetries != 0 {
		t.Errorf("expected explicit zero to be preserved, got %d", cfg.Decompose.MaxValidationRetries)
	}
}

// TestProjectGromitYAML_DecomposeSection verifies the project's gromit.yaml includes
// decompose section with explanatory comments
func TestProjectGromitYAML_DecomposeSection(t *testing.T) {
	// Expected failure: gromit.yaml does not currently have decompose section
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	yamlContent, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", cfgPath, err)
	}

	content := string(yamlContent)

	t.Run("decompose_section_exists", func(t *testing.T) {
		// Expected failure: decompose: section does not exist in gromit.yaml yet
		if !strings.Contains(content, "decompose:") {
			t.Fatal("gromit.yaml missing 'decompose:' section")
		}
	})

	t.Run("validate_field_documented", func(t *testing.T) {
		// Expected failure: validate field and comment do not exist yet
		if !strings.Contains(content, "validate:") {
			t.Error("gromit.yaml decompose section missing 'validate:' field")
		}
		// Verify there's a comment explaining what validation does
		if !strings.Contains(content, "# Check bead definitions") && !strings.Contains(content, "# Validate bead") {
			t.Error("gromit.yaml decompose section missing explanatory comment for validation")
		}
	})

	t.Run("max_validation_retries_field_documented", func(t *testing.T) {
		// Expected failure: max_validation_retries field and comment do not exist yet
		if !strings.Contains(content, "max_validation_retries:") {
			t.Error("gromit.yaml decompose section missing 'max_validation_retries:' field")
		}
		// Verify there's a comment explaining retry behavior
		if !strings.Contains(content, "# How many times") && !strings.Contains(content, "# Retry") {
			t.Error("gromit.yaml decompose section missing explanatory comment for max_validation_retries")
		}
	})

	t.Run("decompose_config_parses", func(t *testing.T) {
		// Expected failure: Config.Decompose field does not exist yet
		cfg, err := Load(cfgPath)
		if err != nil {
			t.Fatalf("Load(%s) error = %v", cfgPath, err)
		}

		// Just verify the section parses without error - values are project-specific
		// Expected failure: Config.Decompose field and IsValidateEnabled method do not exist yet
		_ = cfg.Decompose.IsValidateEnabled()
		_ = cfg.Decompose.MaxValidationRetries
	})
}
