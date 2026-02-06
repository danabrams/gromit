package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Models     ModelsConfig     `yaml:"models"`
	Escalation EscalationConfig `yaml:"escalation"`
	Loop       LoopConfig       `yaml:"loop"`
	Validation ValidationConfig `yaml:"validation"`
	ScopeCheck ScopeCheckConfig `yaml:"scope_check"`
	Preflight  PreflightConfig  `yaml:"preflight"`
	Claude     ClaudeConfig     `yaml:"claude"`
	Paths      PathsConfig      `yaml:"paths"`
}

type ModelsConfig struct {
	P0         string            `yaml:"p0"`
	P1         string            `yaml:"p1"`
	P2         string            `yaml:"p2"`
	Validation string            `yaml:"validation"`
	Labels     map[string]string `yaml:"labels"`
}

type EscalationConfig struct {
	Enabled            bool     `yaml:"enabled"`
	Chain              []string `yaml:"chain"`
	MaxRetriesPerModel int      `yaml:"max_retries_per_model"`
	MaxRetriesPerBead  int      `yaml:"max_retries_per_bead"`
}

type LoopConfig struct {
	MaxIterations     int `yaml:"max_iterations"`
	StopOnFailure     bool `yaml:"stop_on_failure"`
	StuckBeadThreshold int `yaml:"stuck_bead_threshold"`
}

type ValidationConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Commands []string `yaml:"commands"`
}

type ScopeCheckConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`
}

type PreflightConfig struct {
	AutoInstall string   `yaml:"auto_install"` // ask | always | never
	Tools       []string `yaml:"tools"`        // optional explicit list
}

type ClaudeConfig struct {
	Binary             string   `yaml:"binary"`
	Timeout            int      `yaml:"timeout"`
	StallTimeout       int      `yaml:"stall_timeout"`
	StallTimeoutActive int      `yaml:"stall_timeout_active"`
	BeadTimeout        int      `yaml:"bead_timeout"`
	AnalysisTimeout    int      `yaml:"analysis_timeout"`
	Flags              []string `yaml:"flags"`
}

type PathsConfig struct {
	RalphDir        string `yaml:"ralph_dir"`
	Templates       string `yaml:"templates"`
	Specs           string `yaml:"specs"`
	Logs            string `yaml:"logs"`
	ProjectClaudeMD string `yaml:"project_claude_md"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	cfg.setDefaults()
	cfg.normalizeNilFields()
	return &cfg, nil
}

// normalizeNilFields ensures nil slices and maps are replaced with empty
// instances. This prevents issues with downstream code that marshals to JSON
// (nil → "null" vs [] → "[]") and ensures consistent behavior.
func (c *Config) normalizeNilFields() {
	if c.Escalation.Chain == nil {
		c.Escalation.Chain = []string{}
	}
	if c.Validation.Commands == nil {
		c.Validation.Commands = []string{}
	}
	if c.Preflight.Tools == nil {
		c.Preflight.Tools = []string{}
	}
	if c.Claude.Flags == nil {
		c.Claude.Flags = []string{}
	}
	if c.Models.Labels == nil {
		c.Models.Labels = make(map[string]string)
	}
}

func (c *Config) setDefaults() {
	if c.Models.P0 == "" {
		c.Models.P0 = "opus"
	}
	if c.Models.P1 == "" {
		c.Models.P1 = "sonnet"
	}
	if c.Models.P2 == "" {
		c.Models.P2 = "haiku"
	}
	if c.Models.Validation == "" {
		c.Models.Validation = "haiku"
	}
	if c.Claude.Binary == "" {
		c.Claude.Binary = "claude"
	}
	if c.Claude.Timeout == 0 {
		c.Claude.Timeout = 600
	}
	if c.Claude.StallTimeout == 0 {
		c.Claude.StallTimeout = 120
	}
	if c.Claude.StallTimeoutActive == 0 {
		c.Claude.StallTimeoutActive = 300
	}
	if c.Claude.BeadTimeout == 0 {
		c.Claude.BeadTimeout = 1200 // 20 minutes max per bead
	}
	if c.Claude.AnalysisTimeout == 0 {
		c.Claude.AnalysisTimeout = 120 // 2 minutes for failure analysis
	}
	if c.Paths.RalphDir == "" {
		c.Paths.RalphDir = ".ralph"
	}
	if c.Paths.Templates == "" {
		c.Paths.Templates = ".ralph/templates"
	}
	if c.Paths.Specs == "" {
		c.Paths.Specs = ".ralph/specs"
	}
	if c.Paths.Logs == "" {
		c.Paths.Logs = ".ralph/logs"
	}
	if c.Paths.ProjectClaudeMD == "" {
		c.Paths.ProjectClaudeMD = "CLAUDE.md"
	}
	if len(c.Escalation.Chain) == 0 {
		c.Escalation.Chain = []string{"haiku", "sonnet", "opus"}
	}
	if c.Escalation.MaxRetriesPerModel == 0 {
		c.Escalation.MaxRetriesPerModel = 1
	}
	if c.Escalation.MaxRetriesPerBead == 0 {
		c.Escalation.MaxRetriesPerBead = 10
	}
	if c.Preflight.AutoInstall == "" {
		c.Preflight.AutoInstall = "ask"
	}
	if c.Models.Labels == nil {
		c.Models.Labels = make(map[string]string)
	}
	if c.ScopeCheck.Model == "" {
		c.ScopeCheck.Model = "haiku"
	}
	if c.Loop.StuckBeadThreshold == 0 {
		c.Loop.StuckBeadThreshold = 3
	}
}

// SelectModel determines the appropriate model for a bead based on priority and labels
func (c *Config) SelectModel(priority int, labels []string) string {
	if c == nil {
		return "sonnet"
	}
	// Check label overrides first (higher precedence)
	for _, label := range labels {
		if model, ok := c.Models.Labels[label]; ok {
			return model
		}
	}

	// Fall back to priority-based selection
	switch priority {
	case 0:
		return c.Models.P0
	case 1:
		return c.Models.P1
	case 2:
		return c.Models.P2
	default:
		return c.Models.P1 // Default to sonnet for unknown priorities
	}
}

// NextEscalationModel returns the next model in the escalation chain, or empty if at end
func (c *Config) NextEscalationModel(currentModel string) string {
	if c == nil {
		return ""
	}
	if !c.Escalation.Enabled {
		return ""
	}

	for i, model := range c.Escalation.Chain {
		if model == currentModel && i+1 < len(c.Escalation.Chain) {
			return c.Escalation.Chain[i+1]
		}
	}
	return ""
}
