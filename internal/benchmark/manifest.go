package benchmark

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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

	return manifest, nil
}
