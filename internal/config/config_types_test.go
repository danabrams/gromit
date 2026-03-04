package config

import (
	"strings"
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

func TestConfigProjectRootNotSerializedToYAML(t *testing.T) {
	cfg := Config{}
	cfg.ProjectRoot = "/home/user/myproject"
	cfg.Project.Profile = "go"

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	yamlStr := string(data)
	if containsString(yamlStr, "ProjectRoot") {
		t.Fatalf("ProjectRoot should not be serialized to YAML, got: %s", yamlStr)
	}
	if !containsString(yamlStr, "profile") {
		t.Fatalf("profile field should be serialized to YAML, got: %s", yamlStr)
	}
}

func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
