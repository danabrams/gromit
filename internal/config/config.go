package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Model name constants
const (
	ModelOpus   = "opus"
	ModelSonnet = "sonnet"
	ModelHaiku  = "haiku"
)

type Config struct {
	Models      ModelsConfig      `yaml:"models"`
	Escalation  EscalationConfig  `yaml:"escalation"`
	Loop        LoopConfig        `yaml:"loop"`
	Validation  ValidationConfig  `yaml:"validation"`
	ScopeCheck  ScopeCheckConfig  `yaml:"scope_check"`
	Precheck    PrecheckConfig    `yaml:"precheck"`
	Preflight   PreflightConfig   `yaml:"preflight"`
	Claude      ClaudeConfig      `yaml:"claude"`
	Paths       PathsConfig       `yaml:"paths"`
	Review      ReviewConfig      `yaml:"review"`
	Methodology MethodologyConfig `yaml:"methodology"`
	Git         GitConfig         `yaml:"git"`
	State       StateConfig       `yaml:"state"`
	Agents      AgentsConfig      `yaml:"agents"`
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
	MaxIterations            int    `yaml:"max_iterations"`
	StopOnFailure            bool   `yaml:"stop_on_failure"`
	StuckBeadThreshold       int    `yaml:"stuck_bead_threshold"`
	MaxConsecutiveSkips      int    `yaml:"max_consecutive_skips"`
	LearnFromSuccess         *bool  `yaml:"learn_from_success"`
	BetweenIterationsCommand string `yaml:"between_iterations_command"`
}

type ValidationConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Commands []string `yaml:"commands"`
}

type ScopeCheckConfig struct {
	Enabled bool   `yaml:"enabled"`
	Model   string `yaml:"model"`
}

type PrecheckConfig struct {
	Enabled        *bool  `yaml:"enabled"`
	Model          string `yaml:"model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
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
	GromitDir       string `yaml:"gromit_dir"`
	Templates       string `yaml:"templates"`
	Specs           string `yaml:"specs"`
	Plans           string `yaml:"plans"`
	Logs            string `yaml:"logs"`
	ProjectClaudeMD string `yaml:"project_claude_md"`
}

type ReviewConfig struct {
	Enabled         bool                 `yaml:"enabled"`
	Model           string               `yaml:"model"`
	MatchBuildModel *bool                `yaml:"match_build_model"`
	Timeout         int                  `yaml:"timeout"`
	Thorough        ThoroughReviewConfig `yaml:"thorough"`
}

type ThoroughReviewConfig struct {
	Enabled          bool   `yaml:"enabled"`
	EveryNIterations int    `yaml:"every_n_iterations"`
	OnEpicComplete   *bool  `yaml:"on_epic_complete"`
	Model            string `yaml:"model"`
	Timeout          int    `yaml:"timeout"`
}

type MethodologyConfig struct {
	ATDD bool `yaml:"atdd"`
	TDD  bool `yaml:"tdd"`
}

type GitConfig struct {
	AutoPush    *bool  `yaml:"auto_push"`
	PushFailure string `yaml:"push_failure"`
}

type StateConfig struct {
	StaleThreshold int `yaml:"stale_threshold"`
}

type AgentsConfig struct {
	Definitions map[string]AgentDefinition `yaml:"definitions"`
	Phases      PhasesConfig               `yaml:"phases"`
	Prompt      bool                       `yaml:"prompt"`
}

type AgentDefinition struct {
	Binary string   `yaml:"binary"`
	Flags  []string `yaml:"flags"`
}

type PhasesConfig struct {
	Refine  string `yaml:"refine"`
	Plan    string `yaml:"plan"`
	Review  string `yaml:"review"`
	Explore string `yaml:"explore"`
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

	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	return &cfg, nil
}

// NormalizeNilFields ensures nil slices and maps are replaced with empty
// instances. This prevents issues with downstream code that marshals to JSON
// (nil → "null" vs [] → "[]") and ensures consistent behavior.
func (c *Config) NormalizeNilFields() {
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
	if c.Agents.Definitions == nil {
		c.Agents.Definitions = make(map[string]AgentDefinition)
	}
	for name, def := range c.Agents.Definitions {
		if def.Flags == nil {
			def.Flags = []string{}
			c.Agents.Definitions[name] = def
		}
	}
}

// SetDefaults applies default values for all configuration fields.
// This ensures config has sensible defaults even when partially initialized,
// particularly important for test configs created with NewRunnerWithDeps.
func (c *Config) SetDefaults() {
	if c.Models.P0 == "" {
		c.Models.P0 = ModelOpus
	}
	if c.Models.P1 == "" {
		c.Models.P1 = ModelSonnet
	}
	if c.Models.P2 == "" {
		c.Models.P2 = ModelHaiku
	}
	if c.Models.Validation == "" {
		c.Models.Validation = ModelHaiku
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
	if c.Paths.GromitDir == "" {
		c.Paths.GromitDir = ".gromit"
	}
	if c.Paths.Templates == "" {
		c.Paths.Templates = ".gromit/templates"
	}
	if c.Paths.Specs == "" {
		c.Paths.Specs = ".gromit/specs"
	}
	if c.Paths.Plans == "" {
		c.Paths.Plans = ".gromit/plans"
	}
	if c.Paths.Logs == "" {
		c.Paths.Logs = ".gromit/logs"
	}
	if c.Paths.ProjectClaudeMD == "" {
		c.Paths.ProjectClaudeMD = "CLAUDE.md"
	}
	if len(c.Escalation.Chain) == 0 {
		c.Escalation.Chain = []string{ModelHaiku, ModelSonnet, ModelOpus}
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
		c.ScopeCheck.Model = ModelHaiku
	}
	if c.Precheck.Enabled == nil {
		t := true
		c.Precheck.Enabled = &t
	}
	if c.Precheck.Model == "" {
		c.Precheck.Model = ModelHaiku
	}
	if c.Precheck.TimeoutSeconds == 0 {
		c.Precheck.TimeoutSeconds = 30
	}
	if c.Loop.StuckBeadThreshold == 0 {
		c.Loop.StuckBeadThreshold = 3
	}
	if c.Loop.MaxConsecutiveSkips == 0 {
		c.Loop.MaxConsecutiveSkips = 3
	}
	if c.Loop.LearnFromSuccess == nil {
		t := true
		c.Loop.LearnFromSuccess = &t
	}
	if c.Review.Model == "" {
		c.Review.Model = ModelSonnet
	}
	if c.Review.MatchBuildModel == nil {
		t := true
		c.Review.MatchBuildModel = &t
	}
	if c.Review.Timeout == 0 {
		c.Review.Timeout = 120
	}
	if c.Review.Thorough.Model == "" {
		c.Review.Thorough.Model = ModelOpus
	}
	if c.Review.Thorough.EveryNIterations == 0 {
		c.Review.Thorough.EveryNIterations = 5
	}
	if c.Review.Thorough.OnEpicComplete == nil {
		t := true
		c.Review.Thorough.OnEpicComplete = &t
	}
	if c.Review.Thorough.Timeout == 0 {
		c.Review.Thorough.Timeout = 900
	}
	if c.Git.AutoPush == nil {
		t := true
		c.Git.AutoPush = &t
	}
	if c.Git.PushFailure == "" {
		c.Git.PushFailure = "warn"
	}
	if c.State.StaleThreshold == 0 {
		c.State.StaleThreshold = 60
	}
	if c.Agents.Phases.Refine == "" {
		c.Agents.Phases.Refine = "claude"
	}
	if c.Agents.Phases.Plan == "" {
		c.Agents.Phases.Plan = "claude"
	}
	if c.Agents.Phases.Review == "" {
		c.Agents.Phases.Review = "claude"
	}
	if c.Agents.Phases.Explore == "" {
		c.Agents.Phases.Explore = "claude"
	}
}

// SelectModel determines the appropriate model for a bead based on priority and labels
func (c *Config) SelectModel(priority int, labels []string) string {
	if c == nil {
		return ModelSonnet
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

// ShouldMatchBuildModel returns whether review should use the same model as build (opus only)
func (r ReviewConfig) ShouldMatchBuildModel() bool {
	if r.MatchBuildModel == nil {
		return true
	}
	return *r.MatchBuildModel
}

// ShouldRunOnEpicComplete returns whether thorough review should run when an epic completes
func (t ThoroughReviewConfig) ShouldRunOnEpicComplete() bool {
	if t.OnEpicComplete == nil {
		return true
	}
	return *t.OnEpicComplete
}

// ShouldLearnFromSuccess returns whether to extract learnings from successful iterations
func (l LoopConfig) ShouldLearnFromSuccess() bool {
	if l.LearnFromSuccess == nil {
		return true
	}
	return *l.LearnFromSuccess
}

// IsEnabled returns whether precheck should run (defaults to true)
func (p PrecheckConfig) IsEnabled() bool {
	if p.Enabled == nil {
		return true
	}
	return *p.Enabled
}

// IsAutoPushEnabled returns whether git auto-push should run after bead completion (defaults to true)
func (g GitConfig) IsAutoPushEnabled() bool {
	if g.AutoPush == nil {
		return true
	}
	return *g.AutoPush
}
