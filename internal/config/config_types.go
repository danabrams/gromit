package config

import (
	"strings"
	"time"

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

	DefaultRunbookTTLDays     = 14
	DefaultMaxTDDCycles       = 10
	DefaultSpecGateMaxCycles  = 3
	DefaultSpecGateMaxRetries = 3

	DefaultAndonConfigDocSectionTitle = "# Andon autonomy controls"

	MethodologyGranularityBead = "bead"
	MethodologyGranularitySpec = "spec"
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
	EndOfLoopCommand         string `yaml:"end_of_loop_command"`
	MaxDecomposeDepth        int    `yaml:"max_decompose_depth"`
}

type ValidationConfig struct {
	Enabled              bool          `yaml:"enabled"`
	Commands             []string      `yaml:"commands"`
	FastCommands         []string      `yaml:"fast_commands"`
	FullCommands         []string      `yaml:"full_commands"`
	MandatoryCommands    []string      `yaml:"mandatory_commands"`
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
	AutoInstall    string   `yaml:"auto_install"`    // ask | always | never
	Tools          []string `yaml:"tools"`           // optional explicit list
	CompileCommand string   `yaml:"compile_command"` // shell command to run before each invocation
}

type ClaudeConfig struct {
	Binary                 string                           `yaml:"binary"`
	Timeout                int                              `yaml:"timeout"`
	StallTimeout           int                              `yaml:"stall_timeout"`
	StallTimeoutActive     int                              `yaml:"stall_timeout_active"`
	BeadTimeout            int                              `yaml:"bead_timeout"`
	AnalysisTimeout        int                              `yaml:"analysis_timeout"`
	MaxFailureContextChars int                              `yaml:"max_failure_context_chars"`
	MaxInputTokensPerBead  int                              `yaml:"max_input_tokens_per_bead"`
	Flags                  []string                         `yaml:"flags"`
	ModelTimeouts          map[string]ModelTimeoutOverrides `yaml:"model_timeouts"`
}

// ModelTimeoutOverrides allows per-model timeout tuning.
// Non-zero values override the corresponding top-level ClaudeConfig defaults.
type ModelTimeoutOverrides struct {
	Timeout                int `yaml:"timeout"`
	StallTimeout           int `yaml:"stall_timeout"`
	StallTimeoutActive     int `yaml:"stall_timeout_active"`
	BeadTimeout            int `yaml:"bead_timeout"`
	MaxFailureContextChars int `yaml:"max_failure_context_chars"`
	MaxInputTokensPerBead  int `yaml:"max_input_tokens_per_bead"`
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
	ATDD                 bool                    `yaml:"atdd"`
	TDD                  bool                    `yaml:"tdd"`
	MaxTDDCycles         int                     `yaml:"max_tdd_cycles"`
	SpecGateMaxRetries   int                     `yaml:"spec_gate_max_retries"`
	ATDDPrompt           ATDDPromptConfig        `yaml:"atdd_prompt"`
	FreshContextPerCycle bool                    `yaml:"fresh_context_per_cycle"`
	Granularity          string                  `yaml:"granularity"`
	PhaseTimeouts        MethodologyPhaseTimeout `yaml:"phase_timeouts"`
}

type ATDDPromptConfig struct {
	IncludeRules              bool `yaml:"include_rules"`
	IncludeSpec               bool `yaml:"include_spec"`
	IncludeClaudeMD           bool `yaml:"include_claude_md"`
	MaxChars                  int  `yaml:"max_chars"`
	MaxConfirmedLearningChars int  `yaml:"max_confirmed_learning_chars"`

	includeRulesSet              bool `yaml:"-"`
	includeSpecSet               bool `yaml:"-"`
	includeClaudeMDSet           bool `yaml:"-"`
	maxCharsSet                  bool `yaml:"-"`
	maxConfirmedLearningCharsSet bool `yaml:"-"`
}

func (c *ATDDPromptConfig) UnmarshalYAML(value *yaml.Node) error {
	type atddPromptDecode struct {
		IncludeRules              *bool `yaml:"include_rules"`
		IncludeSpec               *bool `yaml:"include_spec"`
		IncludeClaudeMD           *bool `yaml:"include_claude_md"`
		MaxChars                  *int  `yaml:"max_chars"`
		MaxConfirmedLearningChars *int  `yaml:"max_confirmed_learning_chars"`
	}

	var decoded atddPromptDecode
	if err := value.Decode(&decoded); err != nil {
		return err
	}

	if decoded.IncludeRules != nil {
		c.IncludeRules = *decoded.IncludeRules
		c.includeRulesSet = true
	}
	if decoded.IncludeSpec != nil {
		c.IncludeSpec = *decoded.IncludeSpec
		c.includeSpecSet = true
	}
	if decoded.IncludeClaudeMD != nil {
		c.IncludeClaudeMD = *decoded.IncludeClaudeMD
		c.includeClaudeMDSet = true
	}
	if decoded.MaxChars != nil {
		c.MaxChars = *decoded.MaxChars
		c.maxCharsSet = true
	}
	if decoded.MaxConfirmedLearningChars != nil {
		c.MaxConfirmedLearningChars = *decoded.MaxConfirmedLearningChars
		c.maxConfirmedLearningCharsSet = true
	}

	return nil
}

type MethodologyPhaseTimeout struct {
	RedSeconds      int `yaml:"red_seconds"`
	GreenSeconds    int `yaml:"green_seconds"`
	RefactorSeconds int `yaml:"refactor_seconds"`
}

type GitConfig struct {
	AutoPush    *bool  `yaml:"auto_push"`
	PushFailure string `yaml:"push_failure"`
	PushTimeout int    `yaml:"push_timeout"`
}

type StateConfig struct {
	StaleThreshold int `yaml:"stale_threshold"`
}

type LearningsConfig struct {
	MaxLearningChars   int  `yaml:"max_learning_chars"`
	SkipBuildLearnings bool `yaml:"skip_build_learnings"`
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

// ModelCost holds per-model pricing overrides within a provider.
type ModelCost struct {
	CostPer1kInput  float64 `yaml:"cost_per_1k_input"`
	CostPer1kOutput float64 `yaml:"cost_per_1k_output"`
}

type ProviderDef struct {
	Binary          string                `yaml:"binary"`
	Flags           []string              `yaml:"flags"`
	PromptDelivery  string                `yaml:"prompt_delivery"`
	PromptFlag      string                `yaml:"prompt_flag"`
	Models          map[string]string     `yaml:"models"`
	ReasoningEffort map[string]string     `yaml:"reasoning_effort"`
	CostPer1kInput  float64               `yaml:"cost_per_1k_input"`
	CostPer1kOutput float64               `yaml:"cost_per_1k_output"`
	ModelCosts      map[string]*ModelCost `yaml:"model_costs"`
}

// EstimateCost returns an estimated cost in USD based on token counts and
// per-1k-token pricing. If a model-specific rate exists, it takes precedence
// over the provider-level rate. Returns 0 if pricing is not configured or tokens are zero.
func (p ProviderDef) EstimateCost(inputTokens, outputTokens int) float64 {
	return p.EstimateCostForModel("", inputTokens, outputTokens)
}

// EstimateCostForModel returns an estimated cost using model-specific pricing
// when available, falling back to provider-level pricing.
func (p ProviderDef) EstimateCostForModel(model string, inputTokens, outputTokens int) float64 {
	if inputTokens == 0 && outputTokens == 0 {
		return 0
	}
	costIn, costOut := p.CostPer1kInput, p.CostPer1kOutput
	if model != "" && p.ModelCosts != nil {
		if mc, ok := p.ModelCosts[model]; ok && mc != nil {
			costIn, costOut = mc.CostPer1kInput, mc.CostPer1kOutput
		}
	}
	if costIn == 0 && costOut == 0 {
		return 0
	}
	return float64(inputTokens)/tokensPer1k*costIn + float64(outputTokens)/tokensPer1k*costOut
}

type RoutingConfig struct {
	PhasePreferences map[string]string    `yaml:"phase_preferences"`
	Ratio            map[string]int       `yaml:"ratio"`
	Fallback         FallbackConfig       `yaml:"fallback"`
	CircuitBreaker   CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type FallbackConfig struct {
	Enabled  *bool  `yaml:"enabled"`
	Cooldown string `yaml:"cooldown"`
}

type CircuitBreakerConfig struct {
	Enabled           bool    `yaml:"enabled"`
	WindowSize        int     `yaml:"window_size"`
	FailureThreshold  float64 `yaml:"failure_threshold"`
	DegradedFloor     int     `yaml:"degraded_floor"`
	RecoverySuccesses int     `yaml:"recovery_successes"`
}

type StreamConfig struct {
	// PreserveProviderOutput keeps provider-native terminal rendering for stream
	// output (colors/layout) instead of forcing structured event parsing.
	PreserveProviderOutput *bool `yaml:"preserve_provider_output"`
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
	Enabled     *bool  `yaml:"enabled"`
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
