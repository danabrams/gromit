package config

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestGateConfigEffectiveAutoGenerateCriteriaDefaultsToFalse(t *testing.T) {
	var cfg Config
	if cfg.Gate.EffectiveAutoGenerateCriteria() {
		t.Fatalf("EffectiveAutoGenerateCriteria() = true, want false when unspecified")
	}
}

func TestGateConfigEffectiveAutoGenerateCriteriaTracksYAML(t *testing.T) {
	const data = `
gate:
  auto_generate_criteria: true
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if cfg.Gate.AutoGenerateCriteria == nil {
		t.Fatalf("AutoGenerateCriteria = nil, want non-nil pointer")
	}
	if !cfg.Gate.EffectiveAutoGenerateCriteria() {
		t.Fatalf("EffectiveAutoGenerateCriteria() = false, want true when YAML enables it")
	}

	cfg = Config{}
	const disableData = `
gate:
  auto_generate_criteria: false
`
	if err := yaml.Unmarshal([]byte(disableData), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if cfg.Gate.EffectiveAutoGenerateCriteria() {
		t.Fatalf("EffectiveAutoGenerateCriteria() = true, want false when YAML disables it")
	}
}

func TestConfigMidBuildReviewTimeout(t *testing.T) {
	t.Parallel()

	cfg := Config{}
	cfg.MidBuildReview.Timeout = DurationSeconds(30 * time.Second)

	if got := cfg.MidBuildReviewTimeout(); got != 30*time.Second {
		t.Fatalf("MidBuildReviewTimeout() = %v, want %v", got, 30*time.Second)
	}
}
