package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/provider"
	"gopkg.in/yaml.v3"
)

// Model name constants
const (
	ModelOpus   = "opus"
	ModelSonnet = "sonnet"
	ModelHaiku  = "haiku"

	tokensPer1k = 1000.0

	DefaultInvocationTimeoutSeconds = 900 // 15 minutes

	DefaultAndonAssumptionBudget = 2
	DefaultAndonL1RetryCap       = 2
	DefaultAndonL1TimeCapMinutes = 2
	DefaultAndonL2TimeCapMinutes = 15

	DefaultRunbookTTLDays = 14

	DefaultAndonConfigDocSectionTitle = "# Andon autonomy controls"
)

var defaultAndonBulkDeleteAllowlist = []string{
	".gromit/logs/**",
	".gromit/tmp/**",
}

type Config struct {
	Models      ModelsConfig           `yaml:"models"`
	Escalation  EscalationConfig       `yaml:"escalation"`
	Andon       AndonConfig            `yaml:"andon"`
	Loop        LoopConfig             `yaml:"loop"`
	Validation  ValidationConfig       `yaml:"validation"`
	Refactor    RefactorConfig         `yaml:"refactor"`
	ScopeCheck  ScopeCheckConfig       `yaml:"scope_check"`
	Precheck    PrecheckConfig         `yaml:"precheck"`
	Preflight   PreflightConfig        `yaml:"preflight"`
	Claude      ClaudeConfig           `yaml:"claude"`
	Paths       PathsConfig            `yaml:"paths"`
	Review      ReviewConfig           `yaml:"review"`
	Methodology MethodologyConfig      `yaml:"methodology"`
	Git         GitConfig              `yaml:"git"`
	State       StateConfig            `yaml:"state"`
	Learnings   LearningsConfig        `yaml:"learnings"`
	Prompt      PromptConfig           `yaml:"prompt"`
	Agents      AgentsConfig           `yaml:"agents"`
	Providers   map[string]ProviderDef `yaml:"providers"`
	Routing     RoutingConfig          `yaml:"routing"`
	Stream      StreamConfig           `yaml:"stream"`
	Worktree    WorktreeConfig         `yaml:"worktree"`
	Session     SessionConfig          `yaml:"session"`
	Runbook     RunbookConfig          `yaml:"runbook"`
	SpecGate    SpecGateConfig         `yaml:"spec_gate"`
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

type AndonConfig struct {
	AssumptionBudget int                  `yaml:"assumption_budget"`
	L1RetryCap       int                  `yaml:"l1_retry_cap"`
	L1TimeCapMinutes int                  `yaml:"l1_time_cap_minutes"`
	L2TimeCapMinutes int                  `yaml:"l2_time_cap_minutes"`
	HardStops        AndonHardStopsConfig `yaml:"hard_stops"`
}

type AndonHardStopsConfig struct {
	BlockBulkDelete             bool     `yaml:"block_bulk_delete"`
	BlockIrreversibleMigrations bool     `yaml:"block_irreversible_migrations"`
	BlockCredentialChanges      bool     `yaml:"block_credential_changes"`
	BulkDeleteAllowlist         []string `yaml:"bulk_delete_allowlist"`

	blockBulkDeleteSet             bool `yaml:"-"`
	blockIrreversibleMigrationsSet bool `yaml:"-"`
	blockCredentialChangesSet      bool `yaml:"-"`
}

func (h *AndonHardStopsConfig) UnmarshalYAML(value *yaml.Node) error {
	type andonHardStopsDecode struct {
		BlockBulkDelete             *bool    `yaml:"block_bulk_delete"`
		BlockIrreversibleMigrations *bool    `yaml:"block_irreversible_migrations"`
		BlockCredentialChanges      *bool    `yaml:"block_credential_changes"`
		BulkDeleteAllowlist         []string `yaml:"bulk_delete_allowlist"`
	}

	var decoded andonHardStopsDecode
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	if decoded.BlockBulkDelete != nil {
		h.BlockBulkDelete = *decoded.BlockBulkDelete
		h.blockBulkDeleteSet = true
	}
	if decoded.BlockIrreversibleMigrations != nil {
		h.BlockIrreversibleMigrations = *decoded.BlockIrreversibleMigrations
		h.blockIrreversibleMigrationsSet = true
	}
	if decoded.BlockCredentialChanges != nil {
		h.BlockCredentialChanges = *decoded.BlockCredentialChanges
		h.blockCredentialChangesSet = true
	}
	if decoded.BulkDeleteAllowlist != nil {
		h.BulkDeleteAllowlist = decoded.BulkDeleteAllowlist
	}

	return nil
}

func defaultBulkDeleteAllowlist() []string {
	allowlist := make([]string, len(defaultAndonBulkDeleteAllowlist))
	copy(allowlist, defaultAndonBulkDeleteAllowlist)
	return allowlist
}

type LoopConfig struct {
	MaxIterations            int    `yaml:"max_iterations"`
	StopOnFailure            bool   `yaml:"stop_on_failure"`
	StuckBeadThreshold       int    `yaml:"stuck_bead_threshold"`
	MaxConsecutiveSkips      int    `yaml:"max_consecutive_skips"`
	MaxCrossRunFailures      int    `yaml:"max_cross_run_failures"`
	LearnFromSuccess         *bool  `yaml:"learn_from_success"`
	BetweenIterationsCommand string `yaml:"between_iterations_command"`
}

type ValidationConfig struct {
	Enabled              bool          `yaml:"enabled"`
	Commands             []string      `yaml:"commands"`
	FastCommands         []string      `yaml:"fast_commands"`
	FullCommands         []string      `yaml:"full_commands"`
	PhaseTimeoutSeconds  int           `yaml:"phase_timeout_seconds"`
	MaxParallelCommands  int           `yaml:"max_parallel_commands"`
	CommandTimeout       time.Duration `yaml:"command_timeout"`
	FullValidationEveryN int           `yaml:"full_validation_every_n_successes"`
	RunFinalFullGate     *bool         `yaml:"run_final_full_gate"`
	NonInteractive       *bool         `yaml:"non_interactive"`
	MaxFixAttempts       int           `yaml:"max_fix_attempts"`
	MaxValidationRetries int           `yaml:"max_validation_retries"`
}

type RefactorConfig struct {
	MinFilesChanged int `yaml:"min_files_changed"`
}

type ScopeCheckConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Model          string `yaml:"model"`
	BlockOversized *bool  `yaml:"block_oversized"`
}

type PrecheckConfig struct {
	Enabled        *bool                      `yaml:"enabled"`
	Model          string                     `yaml:"model"`
	TimeoutSeconds int                        `yaml:"timeout_seconds"`
	Verification   PrecheckVerificationConfig `yaml:"verification"`
}

type PrecheckVerificationConfig struct {
	Enabled        *bool `yaml:"enabled"`
	TimeoutSeconds int   `yaml:"timeout_seconds"`
}

type PreflightConfig struct {
	AutoInstall  string   `yaml:"auto_install"`  // ask | always | never
	Tools        []string `yaml:"tools"`         // optional explicit list
	CompileCheck *bool    `yaml:"compile_check"` // run go build ./... before each invocation
}

type ClaudeConfig struct {
	Binary             string                           `yaml:"binary"`
	Timeout            int                              `yaml:"timeout"`
	StallTimeout       int                              `yaml:"stall_timeout"`
	StallTimeoutActive int                              `yaml:"stall_timeout_active"`
	BeadTimeout        int                              `yaml:"bead_timeout"`
	AnalysisTimeout    int                              `yaml:"analysis_timeout"`
	Flags              []string                         `yaml:"flags"`
	ModelTimeouts      map[string]ModelTimeoutOverrides `yaml:"model_timeouts"`
}

// ModelTimeoutOverrides allows per-model timeout tuning.
// Non-zero values override the corresponding top-level ClaudeConfig defaults.
type ModelTimeoutOverrides struct {
	Timeout            int `yaml:"timeout"`
	StallTimeout       int `yaml:"stall_timeout"`
	StallTimeoutActive int `yaml:"stall_timeout_active"`
	BeadTimeout        int `yaml:"bead_timeout"`
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
	ATDD          bool                    `yaml:"atdd"`
	TDD           bool                    `yaml:"tdd"`
	PhaseTimeouts MethodologyPhaseTimeout `yaml:"phase_timeouts"`
}

type MethodologyPhaseTimeout struct {
	RedSeconds      int `yaml:"red_seconds"`
	GreenSeconds    int `yaml:"green_seconds"`
	RefactorSeconds int `yaml:"refactor_seconds"`
}

type GitConfig struct {
	AutoPush    *bool  `yaml:"auto_push"`
	PushFailure string `yaml:"push_failure"`
}

type StateConfig struct {
	StaleThreshold int `yaml:"stale_threshold"`
}

type LearningsConfig struct {
	MaxLearningChars int `yaml:"max_learning_chars"`
}

type PromptConfig struct {
	Budget PromptBudgetConfig `yaml:"budget"`
}

type PromptBudgetConfig struct {
	MaxChars         int `yaml:"max_chars"`
	LearningCapChars int `yaml:"learning_cap_chars"`
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
	Debug   string `yaml:"debug"`
}

type ProviderDef struct {
	Binary          string            `yaml:"binary"`
	Flags           []string          `yaml:"flags"`
	PromptDelivery  string            `yaml:"prompt_delivery"`
	PromptFlag      string            `yaml:"prompt_flag"`
	Models          map[string]string `yaml:"models"`
	CostPer1kInput  float64           `yaml:"cost_per_1k_input"`
	CostPer1kOutput float64           `yaml:"cost_per_1k_output"`
}

// EstimateCost returns an estimated cost in USD based on token counts and
// per-1k-token pricing. Returns 0 if pricing is not configured or tokens are zero.
func (p ProviderDef) EstimateCost(inputTokens, outputTokens int) float64 {
	if inputTokens == 0 && outputTokens == 0 {
		return 0
	}
	if p.CostPer1kInput == 0 && p.CostPer1kOutput == 0 {
		return 0
	}
	return float64(inputTokens)/tokensPer1k*p.CostPer1kInput + float64(outputTokens)/tokensPer1k*p.CostPer1kOutput
}

type RoutingConfig struct {
	PhasePreferences map[string]string `yaml:"phase_preferences"`
	Ratio            map[string]int    `yaml:"ratio"`
	Fallback         FallbackConfig    `yaml:"fallback"`
}

type FallbackConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Cooldown string `yaml:"cooldown"`
}

type StreamConfig struct {
	// PreserveProviderOutput keeps provider-native terminal rendering for stream
	// output (colors/layout) instead of forcing structured event parsing.
	PreserveProviderOutput *bool `yaml:"preserve_provider_output"`
}

func (s StreamConfig) PreserveProviderOutputEnabled() bool {
	if s.PreserveProviderOutput == nil {
		return true
	}
	return *s.PreserveProviderOutput
}

type WorktreeConfig struct {
	Enabled            *bool  `yaml:"enabled"`
	AutoMerge          *bool  `yaml:"auto_merge"`
	MergeFailure       string `yaml:"merge_failure"`
	ConflictResolution string `yaml:"conflict_resolution"`
	RetryCap           int    `yaml:"retry_cap"`
}

type SessionConfig struct {
	Iterations    int    `yaml:"iterations"`
	TestCommand   string `yaml:"test_command"`
	MaxFixRetries int    `yaml:"max_fix_retries"`
	FixTier       string `yaml:"fix_tier"`
	Review        *bool  `yaml:"review"`
	Retro         *bool  `yaml:"retro"`
}

type RunbookConfig struct {
	TTLDays int `yaml:"ttl_days"`
}

type SpecGateConfig struct {
	Enabled     bool   `yaml:"enabled"`
	MaxCycles   int    `yaml:"max_cycles"`
	Model       string `yaml:"model"`
	AutoTrigger *bool  `yaml:"auto_trigger"`
}

// ResolvePhaseTimeoutSeconds returns the configured timeout for a methodology
// phase, or falls back to beadTimeoutSeconds when unset/zero.
func (m MethodologyConfig) ResolvePhaseTimeoutSeconds(phase string, beadTimeoutSeconds int) int {
	if timeoutSeconds := m.PhaseTimeouts.forPhase(phase); timeoutSeconds > 0 {
		return timeoutSeconds
	}
	return beadTimeoutSeconds
}

func (t MethodologyPhaseTimeout) forPhase(phase string) int {
	switch strings.ToLower(phase) {
	case "red":
		return t.RedSeconds
	case "green":
		return t.GreenSeconds
	case "refactor":
		return t.RefactorSeconds
	default:
		return 0
	}
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
	if c.Validation.FastCommands == nil {
		c.Validation.FastCommands = []string{}
	}
	if c.Validation.FullCommands == nil {
		c.Validation.FullCommands = []string{}
	}
	if c.Andon.HardStops.BulkDeleteAllowlist == nil {
		c.Andon.HardStops.BulkDeleteAllowlist = []string{}
	}
	if c.Preflight.Tools == nil {
		c.Preflight.Tools = []string{}
	}
	if c.Claude.Flags == nil {
		c.Claude.Flags = []string{}
	}
	if c.Claude.ModelTimeouts == nil {
		c.Claude.ModelTimeouts = make(map[string]ModelTimeoutOverrides)
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
	// Normalize Providers fields
	for name, def := range c.Providers {
		if def.Flags == nil {
			def.Flags = []string{}
		}
		if def.Models == nil {
			def.Models = make(map[string]string)
		}
		c.Providers[name] = def
	}
	// Normalize Routing fields
	if c.Routing.PhasePreferences == nil {
		c.Routing.PhasePreferences = make(map[string]string)
	}
	if c.Routing.Ratio == nil {
		c.Routing.Ratio = make(map[string]int)
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
		c.Claude.Timeout = DefaultInvocationTimeoutSeconds
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
	if c.Validation.MaxFixAttempts == 0 {
		c.Validation.MaxFixAttempts = 1
	}
	if c.Validation.MaxValidationRetries == 0 {
		c.Validation.MaxValidationRetries = 2
	}
	if c.Validation.MaxParallelCommands == 0 {
		c.Validation.MaxParallelCommands = 1
	}
	if c.Validation.NonInteractive == nil {
		t := true
		c.Validation.NonInteractive = &t
	}
	if c.Validation.RunFinalFullGate == nil {
		t := true
		c.Validation.RunFinalFullGate = &t
	}
	if c.Refactor.MinFilesChanged == 0 {
		c.Refactor.MinFilesChanged = 3
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
	if c.Andon.AssumptionBudget == 0 {
		c.Andon.AssumptionBudget = DefaultAndonAssumptionBudget
	}
	if c.Andon.L1RetryCap == 0 {
		c.Andon.L1RetryCap = DefaultAndonL1RetryCap
	}
	if c.Andon.L1TimeCapMinutes == 0 {
		c.Andon.L1TimeCapMinutes = DefaultAndonL1TimeCapMinutes
	}
	if c.Andon.L2TimeCapMinutes == 0 {
		c.Andon.L2TimeCapMinutes = DefaultAndonL2TimeCapMinutes
	}
	if !c.Andon.HardStops.blockBulkDeleteSet {
		c.Andon.HardStops.BlockBulkDelete = true
	}
	if !c.Andon.HardStops.blockIrreversibleMigrationsSet {
		c.Andon.HardStops.BlockIrreversibleMigrations = true
	}
	if !c.Andon.HardStops.blockCredentialChangesSet {
		c.Andon.HardStops.BlockCredentialChanges = true
	}
	if len(c.Andon.HardStops.BulkDeleteAllowlist) == 0 {
		c.Andon.HardStops.BulkDeleteAllowlist = defaultBulkDeleteAllowlist()
	}
	if c.Preflight.AutoInstall == "" {
		c.Preflight.AutoInstall = "ask"
	}
	if c.Preflight.CompileCheck == nil {
		t := true
		c.Preflight.CompileCheck = &t
	}
	if c.Models.Labels == nil {
		c.Models.Labels = make(map[string]string)
	}
	if c.ScopeCheck.Model == "" {
		c.ScopeCheck.Model = ModelHaiku
	}
	if c.ScopeCheck.BlockOversized == nil {
		t := true
		c.ScopeCheck.BlockOversized = &t
	}
	if c.Precheck.Enabled == nil {
		t := true
		c.Precheck.Enabled = &t
	}
	if c.Precheck.Model == "" {
		c.Precheck.Model = ModelHaiku
	}
	if c.Precheck.TimeoutSeconds == 0 {
		c.Precheck.TimeoutSeconds = 120
	}
	if c.Precheck.Verification.Enabled == nil {
		t := true
		c.Precheck.Verification.Enabled = &t
	}
	if c.Precheck.Verification.TimeoutSeconds == 0 {
		c.Precheck.Verification.TimeoutSeconds = 120
	}
	if c.Loop.StuckBeadThreshold == 0 {
		c.Loop.StuckBeadThreshold = 3
	}
	if c.Loop.MaxConsecutiveSkips == 0 {
		c.Loop.MaxConsecutiveSkips = 3
	}
	if c.Loop.MaxCrossRunFailures == 0 {
		c.Loop.MaxCrossRunFailures = 5
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
	if c.Learnings.MaxLearningChars == 0 {
		c.Learnings.MaxLearningChars = 8000
	}
	if c.Prompt.Budget.MaxChars == 0 {
		c.Prompt.Budget.MaxChars = 20000
	}
	if c.Prompt.Budget.LearningCapChars == 0 {
		c.Prompt.Budget.LearningCapChars = 2000
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
	if c.Agents.Phases.Debug == "" {
		c.Agents.Phases.Debug = "claude"
	}

	// Routing defaults — only when providers are configured
	if c.HasProviders() {
		// Initialize PhasePreferences if nil
		if c.Routing.PhasePreferences == nil {
			c.Routing.PhasePreferences = make(map[string]string)
		}

		// Build equal-split ratio from provider names when ratio is empty
		if len(c.Routing.Ratio) == 0 {
			c.Routing.Ratio = make(map[string]int)
			share := 100 / len(c.Providers)
			for name := range c.Providers {
				c.Routing.Ratio[name] = share
			}
		}

		// Default fallback cooldown
		if c.Routing.Fallback.Cooldown == "" {
			c.Routing.Fallback.Cooldown = "30m"
		}

		// Enable fallback for multi-provider configs
		if len(c.Providers) > 1 && !c.Routing.Fallback.Enabled {
			c.Routing.Fallback.Enabled = true
		}
	}

	if c.Stream.PreserveProviderOutput == nil {
		t := true
		c.Stream.PreserveProviderOutput = &t
	}

	// Worktree defaults
	if c.Worktree.Enabled == nil {
		t := true
		c.Worktree.Enabled = &t
	}
	if c.Worktree.AutoMerge == nil {
		t := true
		c.Worktree.AutoMerge = &t
	}
	if c.Worktree.MergeFailure == "" {
		c.Worktree.MergeFailure = "warn"
	}
	if c.Worktree.ConflictResolution == "" {
		c.Worktree.ConflictResolution = "abort"
	}
	if c.Worktree.RetryCap == 0 {
		c.Worktree.RetryCap = 3
	}

	if c.Runbook.TTLDays == 0 {
		c.Runbook.TTLDays = DefaultRunbookTTLDays
	}
	if c.Session.MaxFixRetries == 0 {
		c.Session.MaxFixRetries = 3
	}
	if c.Session.FixTier == "" {
		c.Session.FixTier = "medium"
	}
	if c.Session.Review == nil {
		t := true
		c.Session.Review = &t
	}
	if c.Session.Retro == nil {
		t := true
		c.Session.Retro = &t
	}
	if c.SpecGate.MaxCycles == 0 {
		c.SpecGate.MaxCycles = 3
	}
	if c.SpecGate.Model == "" {
		c.SpecGate.Model = ModelSonnet
	}
	if c.SpecGate.AutoTrigger == nil {
		t := true
		c.SpecGate.AutoTrigger = &t
	}
}

func (v ValidationConfig) IsNonInteractive() bool {
	if v.NonInteractive == nil {
		return true
	}
	return *v.NonInteractive
}

// FastCommandsOrDefault returns the per-bead validation command set.
// `fast_commands` takes precedence, then falls back to legacy `commands`.
func (v ValidationConfig) FastCommandsOrDefault() []string {
	if len(v.FastCommands) > 0 {
		return v.FastCommands
	}
	return v.Commands
}

// FullCommandsOrDefault returns the full verification command set.
// `full_commands` takes precedence, then falls back to legacy `commands`.
func (v ValidationConfig) FullCommandsOrDefault() []string {
	if len(v.FullCommands) > 0 {
		return v.FullCommands
	}
	return v.Commands
}

// ShouldRunFinalFullGate returns whether the session-ending full validation
// gate should run (defaults to true).
func (v ValidationConfig) ShouldRunFinalFullGate() bool {
	if v.RunFinalFullGate == nil {
		return true
	}
	return *v.RunFinalFullGate
}

// ResolvePhaseTimeoutSeconds returns validation's phase timeout override, or
// beadTimeoutSeconds when phase_timeout_seconds is unset/zero.
func (v ValidationConfig) ResolvePhaseTimeoutSeconds(beadTimeoutSeconds int) int {
	if v.PhaseTimeoutSeconds > 0 {
		return v.PhaseTimeoutSeconds
	}
	return beadTimeoutSeconds
}

// ScopeGoTestCommands scopes "go test ./..." commands to touched packages.
// Non-go-test commands and commands without "./..." are returned unchanged.
func ScopeGoTestCommands(commands []string, touchedPackages []string) []string {
	if len(commands) == 0 || len(touchedPackages) == 0 {
		return commands
	}

	uniqueTouched := make([]string, 0, len(touchedPackages))
	seen := make(map[string]bool, len(touchedPackages))
	for _, pkg := range touchedPackages {
		trimmed := strings.TrimSpace(pkg)
		normalized := strings.Trim(strings.TrimPrefix(trimmed, "./"), "/")
		if trimmed == "." || strings.TrimSpace(normalized) == "." {
			normalized = "."
		}
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		uniqueTouched = append(uniqueTouched, normalized)
	}

	// Collapse nested package scopes to avoid duplicate work.
	// Example: when both internal/runner and internal/runner/andon are touched,
	// only keep internal/runner because it already includes children via /...
	collapsed := make([]string, 0, len(uniqueTouched))
	for _, pkg := range uniqueTouched {
		coveredByParent := false
		for _, parent := range collapsed {
			if pkg == parent || strings.HasPrefix(pkg, parent+"/") {
				coveredByParent = true
				break
			}
		}
		if coveredByParent {
			continue
		}

		filtered := collapsed[:0]
		for _, existing := range collapsed {
			if existing == pkg || strings.HasPrefix(existing, pkg+"/") {
				continue
			}
			filtered = append(filtered, existing)
		}
		collapsed = append(filtered, pkg)
	}

	scopedPackages := make([]string, 0, len(collapsed))
	for _, pkg := range collapsed {
		if pkg == "." {
			scopedPackages = append(scopedPackages, ".")
			continue
		}
		scopedPackages = append(scopedPackages, "./"+pkg+"/...")
	}
	if len(scopedPackages) == 0 {
		return commands
	}

	scoped := make([]string, 0, len(commands))
	for _, command := range commands {
		fields := strings.Fields(command)
		if len(fields) < 3 || fields[0] != "go" || fields[1] != "test" {
			scoped = append(scoped, command)
			continue
		}

		replaced := false
		rebuilt := make([]string, 0, len(fields)+len(scopedPackages))
		for _, token := range fields {
			if token == "./..." {
				rebuilt = append(rebuilt, scopedPackages...)
				replaced = true
				continue
			}
			rebuilt = append(rebuilt, token)
		}
		if replaced {
			scoped = append(scoped, strings.Join(rebuilt, " "))
			continue
		}
		scoped = append(scoped, command)
	}

	return scoped
}

// IsTierName returns true if the string is a valid tier name (high, medium, low).
// Case-insensitive comparison handles config variations like "High", "MEDIUM", etc.
func (c *Config) IsTierName(s string) bool {
	switch strings.ToLower(s) {
	case "high", "medium", "low":
		return true
	default:
		return false
	}
}

// SelectTier determines the appropriate tier for a bead based on priority and labels.
// Returns abstract tier names (high, medium, low) instead of provider-specific model names.
// For backward compatibility, auto-maps legacy model names via TierFromLegacyModel when
// config contains model names instead of tier names.
func (c *Config) SelectTier(priority int, labels []string) string {
	if c == nil {
		return provider.TierMedium
	}

	// Check label overrides first (higher precedence)
	for _, label := range labels {
		if value, ok := c.Models.Labels[label]; ok {
			// Auto-map legacy model names to tiers
			return provider.TierFromLegacyModel(value)
		}
	}

	// Fall back to priority-based selection
	var value string
	switch priority {
	case 0:
		value = c.Models.P0
	case 1:
		value = c.Models.P1
	case 2:
		value = c.Models.P2
	default:
		value = c.Models.P1 // Default to medium tier for unknown priorities
	}

	// Auto-map legacy model names to tiers
	return provider.TierFromLegacyModel(value)
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

// NextEscalationTier returns the next tier in the escalation chain, or empty if at end.
// For backward compatibility, auto-maps legacy model names in the chain and input to tiers.
func (c *Config) NextEscalationTier(currentTier string) string {
	if c == nil {
		return ""
	}
	if !c.Escalation.Enabled {
		return ""
	}

	// Auto-map the current tier if it's a legacy model name
	mappedCurrentTier := provider.TierFromLegacyModel(currentTier)

	for i, chainEntry := range c.Escalation.Chain {
		// Auto-map chain entry if it's a legacy model name
		mappedChainEntry := provider.TierFromLegacyModel(chainEntry)

		if mappedChainEntry == mappedCurrentTier && i+1 < len(c.Escalation.Chain) {
			// Return the next entry, also mapped
			return provider.TierFromLegacyModel(c.Escalation.Chain[i+1])
		}
	}
	return ""
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

// IsVerificationEnabled returns whether precheck verification should run (defaults to true)
func (v PrecheckVerificationConfig) IsVerificationEnabled() bool {
	if v.Enabled == nil {
		return true
	}
	return *v.Enabled
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

// TimeoutsForModel returns the effective invocation, stall, stall-active, and bead timeouts for a model.
// Per-model overrides (if non-zero) take precedence over the top-level defaults.
func (c ClaudeConfig) TimeoutsForModel(model string) (invocationTimeout, stallTimeout, stallTimeoutActive, beadTimeout int) {
	invocationTimeout = c.Timeout
	stallTimeout = c.StallTimeout
	stallTimeoutActive = c.StallTimeoutActive
	beadTimeout = c.BeadTimeout

	if overrides, ok := c.ModelTimeouts[model]; ok {
		if overrides.Timeout > 0 {
			invocationTimeout = overrides.Timeout
		}
		if overrides.StallTimeout > 0 {
			stallTimeout = overrides.StallTimeout
		}
		if overrides.StallTimeoutActive > 0 {
			stallTimeoutActive = overrides.StallTimeoutActive
		}
		if overrides.BeadTimeout > 0 {
			beadTimeout = overrides.BeadTimeout
		}
	}
	return
}

// ShouldBlockOversized returns whether over-scoped beads should be blocked before execution (defaults to true)
func (s ScopeCheckConfig) ShouldBlockOversized() bool {
	if s.BlockOversized == nil {
		return true
	}
	return *s.BlockOversized
}

// HasProviders returns true when providers section is non-empty
func (c *Config) HasProviders() bool {
	return len(c.Providers) > 0
}

// IsEnabled returns whether worktree isolation is enabled (defaults to true)
func (w WorktreeConfig) IsEnabled() bool {
	if w.Enabled == nil {
		return true
	}
	return *w.Enabled
}

// IsAutoMergeEnabled returns whether automatic merge-back is enabled (defaults to true)
func (w WorktreeConfig) IsAutoMergeEnabled() bool {
	if w.AutoMerge == nil {
		return true
	}
	return *w.AutoMerge
}

// IsEnabled returns whether spec gate is enabled
func (s SpecGateConfig) IsEnabled() bool {
	return s.Enabled
}

// IsAutoTrigger returns whether spec gate auto-trigger is enabled (defaults to true)
func (s SpecGateConfig) IsAutoTrigger() bool {
	if s.AutoTrigger == nil {
		return true
	}
	return *s.AutoTrigger
}
