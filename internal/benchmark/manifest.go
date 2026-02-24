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
	ID              string   `yaml:"id"`
	BaseCommit      string   `yaml:"base_commit"`
	Beads           []string `yaml:"beads"`
	Modes           []string `yaml:"modes"`
	Provider        string   `yaml:"provider"`
	ModelFamily     string   `yaml:"model_family"`
	LowTierModel    string   `yaml:"low_tier_model"`
	MediumTierModel string   `yaml:"medium_tier_model"`
	HighTierModel   string   `yaml:"high_tier_model"`
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
	if manifest.ID == "" {
		return Manifest{}, fmt.Errorf("validate manifest %q: id is required", path)
	}
	if len(manifest.Beads) == 0 {
		return Manifest{}, fmt.Errorf("validate manifest %q: beads is required", path)
	}
	if len(manifest.Modes) == 0 {
		return Manifest{}, fmt.Errorf("validate manifest %q: modes is required", path)
	}
	for _, mode := range manifest.Modes {
		if _, ok := allowedBenchmarkModes[mode]; !ok {
			return Manifest{}, fmt.Errorf("validate manifest %q: unsupported mode %q", path, mode)
		}
	}

	return manifest, nil
}
