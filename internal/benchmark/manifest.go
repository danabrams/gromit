package benchmark

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var allowedBenchmarkModes = map[string]struct{}{
	"single_pass":        {},
	"tdd_shared_context": {},
	"tdd_fresh_context":  {},
}

type Manifest struct {
	ID         string   `yaml:"id"`
	BaseCommit string   `yaml:"base_commit"`
	Beads      []string `yaml:"beads"`
	ModeConfig `yaml:",inline"`
	ModelPinning `yaml:",inline"`
}

type ModeConfig struct {
	Modes []string `yaml:"modes"`
}

type ModelPinning struct {
	Provider        string `yaml:"provider"`
	ModelFamily     string `yaml:"model_family"`
	LowTierModel    string `yaml:"low_tier_model"`
	MediumTierModel string `yaml:"medium_tier_model"`
	HighTierModel   string `yaml:"high_tier_model"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %q: %w", path, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, fmt.Errorf("validate manifest %q: %w", path, err)
	}

	return manifest, nil
}

func ValidateManifest(manifest Manifest) error {
	if manifest.ID == "" {
		return fmt.Errorf("id is required")
	}
	if len(manifest.Beads) == 0 {
		return fmt.Errorf("beads is required")
	}
	if len(manifest.Modes) == 0 {
		return fmt.Errorf("modes is required")
	}
	if manifest.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if manifest.ModelFamily == "" {
		return fmt.Errorf("model_family is required")
	}
	if manifest.LowTierModel == "" {
		return fmt.Errorf("low_tier_model is required")
	}
	if manifest.MediumTierModel == "" {
		return fmt.Errorf("medium_tier_model is required")
	}
	if manifest.HighTierModel == "" {
		return fmt.Errorf("high_tier_model is required")
	}
	for _, mode := range manifest.Modes {
		if _, ok := allowedBenchmarkModes[mode]; !ok {
			return fmt.Errorf("unsupported mode %q", mode)
		}
	}

	return nil
}
