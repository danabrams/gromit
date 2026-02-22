package config

const defaultPhaseAgent = "claude"

func boolPtr(value bool) *bool {
	return &value
}

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
	if c.Claude.PipelineTimeout == 0 {
		c.Claude.PipelineTimeout = DefaultPipelineTimeoutSeconds
	}
	if c.Claude.StallTimeout == 0 {
		c.Claude.StallTimeout = DefaultStallTimeoutSeconds
	}
	if c.Claude.StallTimeoutActive == 0 {
		c.Claude.StallTimeoutActive = DefaultStallTimeoutActiveSeconds
	}
	if c.Claude.BeadTimeout == 0 {
		c.Claude.BeadTimeout = 1200 // 20 minutes max per bead
	}
	if c.Claude.AnalysisTimeout == 0 {
		c.Claude.AnalysisTimeout = 120 // 2 minutes for failure analysis
	}
	if c.Claude.MaxFailureContextChars == 0 {
		c.Claude.MaxFailureContextChars = 2000
	}
	if c.Claude.MaxInputTokensPerBead == 0 {
		c.Claude.MaxInputTokensPerBead = 400000
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
		c.Validation.NonInteractive = boolPtr(true)
	}
	if c.Validation.RunFinalFullGate == nil {
		c.Validation.RunFinalFullGate = boolPtr(true)
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
	if c.Models.Labels == nil {
		c.Models.Labels = make(map[string]string)
	}
	if c.ScopeCheck.Model == "" {
		c.ScopeCheck.Model = ModelHaiku
	}
	if c.ScopeCheck.BlockOversized == nil {
		c.ScopeCheck.BlockOversized = boolPtr(true)
	}
	if c.Precheck.Enabled == nil {
		c.Precheck.Enabled = boolPtr(false)
	}
	if c.Precheck.Model == "" {
		c.Precheck.Model = ModelHaiku
	}
	if c.Precheck.TimeoutSeconds == 0 {
		c.Precheck.TimeoutSeconds = 120
	}
	if c.Precheck.Verification.Enabled == nil {
		c.Precheck.Verification.Enabled = boolPtr(true)
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
	if c.Loop.MaxDecomposeDepth == 0 {
		c.Loop.MaxDecomposeDepth = 10
	}
	if c.Loop.LearnFromSuccess == nil {
		c.Loop.LearnFromSuccess = boolPtr(true)
	}
	if c.Review.Model == "" {
		c.Review.Model = ModelSonnet
	}
	if c.Review.Tier == "" {
		c.Review.Tier = "medium"
	}
	if c.Review.MatchBuildModel == nil {
		c.Review.MatchBuildModel = boolPtr(true)
	}
	if c.Review.Timeout == 0 {
		c.Review.Timeout = 120
	}
	if c.Review.Thorough.Model == "" {
		c.Review.Thorough.Model = ModelOpus
	}
	if c.Review.Thorough.Tier == "" {
		c.Review.Thorough.Tier = "high"
	}
	if c.Review.Thorough.EveryNIterations == 0 {
		c.Review.Thorough.EveryNIterations = 5
	}
	if c.Review.Thorough.OnEpicComplete == nil {
		c.Review.Thorough.OnEpicComplete = boolPtr(true)
	}
	if c.Review.Thorough.Timeout == 0 {
		c.Review.Thorough.Timeout = 900
	}
	if c.Methodology.Granularity == "" {
		c.Methodology.Granularity = MethodologyGranularityBead
	}
	if c.Methodology.BuildStrategy == "" {
		c.Methodology.BuildStrategy = "single_pass"
	}
	if c.Methodology.PhaseModels.Decompose == "" {
		c.Methodology.PhaseModels.Decompose = "medium"
	}
	if c.Methodology.PhaseModels.Build == "" {
		c.Methodology.PhaseModels.Build = "medium"
	}
	if c.Methodology.PhaseModels.Red == "" {
		c.Methodology.PhaseModels.Red = "low"
	}
	if c.Methodology.PhaseModels.Green == "" {
		c.Methodology.PhaseModels.Green = "medium"
	}
	if c.Methodology.PhaseModels.Refactor == "" {
		c.Methodology.PhaseModels.Refactor = "low"
	}
	if !c.Methodology.ATDDPrompt.includeRulesSet {
		c.Methodology.ATDDPrompt.IncludeRules = true
	}
	if !c.Methodology.ATDDPrompt.includeSpecSet {
		c.Methodology.ATDDPrompt.IncludeSpec = true
	}
	if !c.Methodology.ATDDPrompt.includeClaudeMDSet {
		c.Methodology.ATDDPrompt.IncludeClaudeMD = true
	}
	if !c.Methodology.ATDDPrompt.maxCharsSet {
		c.Methodology.ATDDPrompt.MaxChars = 20000
	}
	if !c.Methodology.ATDDPrompt.maxConfirmedLearningCharsSet {
		c.Methodology.ATDDPrompt.MaxConfirmedLearningChars = 2000
	}
	if c.Methodology.MaxTDDCycles == 0 {
		c.Methodology.MaxTDDCycles = DefaultMaxTDDCycles
	}
	if c.Methodology.SpecGateMaxRetries == 0 {
		c.Methodology.SpecGateMaxRetries = DefaultSpecGateMaxRetries
	}
	if c.Git.AutoPush == nil {
		c.Git.AutoPush = boolPtr(true)
	}
	if c.Git.PushFailure == "" {
		c.Git.PushFailure = "warn"
	}
	if c.Git.PushTimeout == 0 {
		c.Git.PushTimeout = 60
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
		c.Agents.Phases.Refine = defaultPhaseAgent
	}
	if c.Agents.Phases.Plan == "" {
		c.Agents.Phases.Plan = defaultPhaseAgent
	}
	if c.Agents.Phases.Review == "" {
		c.Agents.Phases.Review = defaultPhaseAgent
	}
	if c.Agents.Phases.Explore == "" {
		c.Agents.Phases.Explore = defaultPhaseAgent
	}
	if c.Agents.Phases.Debug == "" {
		c.Agents.Phases.Debug = defaultPhaseAgent
	}

	// Routing defaults - only when providers are configured
	if c.HasProviders() {
		providerCount := len(c.Providers)

		// Initialize PhasePreferences if nil
		if c.Routing.PhasePreferences == nil {
			c.Routing.PhasePreferences = make(map[string]string)
		}

		// Build equal-split ratio from provider names when ratio is empty
		if len(c.Routing.Ratio) == 0 {
			c.Routing.Ratio = make(map[string]int)
			share := 100 / providerCount
			for name := range c.Providers {
				c.Routing.Ratio[name] = share
			}
		}

		// Default fallback cooldown
		if c.Routing.Fallback.Cooldown == "" {
			c.Routing.Fallback.Cooldown = "30m"
		}

		// Enable fallback by default for multi-provider configs.
		if c.Routing.Fallback.Enabled == nil {
			enabled := providerCount > 1
			c.Routing.Fallback.Enabled = &enabled
		}
	}

	if c.Stream.PreserveProviderOutput == nil {
		c.Stream.PreserveProviderOutput = boolPtr(true)
	}

	if c.Routing.CircuitBreaker.Enabled {
		if c.Routing.CircuitBreaker.WindowSize == 0 {
			c.Routing.CircuitBreaker.WindowSize = 10
		}
		if c.Routing.CircuitBreaker.FailureThreshold == 0 {
			c.Routing.CircuitBreaker.FailureThreshold = 0.3
		}
		if c.Routing.CircuitBreaker.DegradedFloor == 0 {
			c.Routing.CircuitBreaker.DegradedFloor = 20
		}
		if c.Routing.CircuitBreaker.RecoverySuccesses == 0 {
			c.Routing.CircuitBreaker.RecoverySuccesses = 5
		}
	}

	// Worktree defaults
	if c.Worktree.Enabled == nil {
		c.Worktree.Enabled = boolPtr(true)
	}
	if c.Worktree.AutoMerge == nil {
		c.Worktree.AutoMerge = boolPtr(true)
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
		c.Session.Review = boolPtr(true)
	}
	if c.Session.Retro == nil {
		c.Session.Retro = boolPtr(true)
	}
	if c.SpecGate.MaxCycles == 0 {
		c.SpecGate.MaxCycles = DefaultSpecGateMaxCycles
	}
	if c.SpecGate.Model == "" {
		c.SpecGate.Model = ModelSonnet
	}
	if c.SpecGate.Enabled == nil {
		c.SpecGate.Enabled = boolPtr(true)
	}
	if c.SpecGate.AutoTrigger == nil {
		c.SpecGate.AutoTrigger = boolPtr(true)
	}
	if c.Decompose.Tier == "" {
		c.Decompose.Tier = "medium"
	}
}
