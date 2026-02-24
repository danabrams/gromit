package config

import (
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var configWarningWriter io.Writer = os.Stderr

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	matchBuildModelConfigured := cfg.Review.MatchBuildModel != nil
	cfg.applyPostLoadNormalization(matchBuildModelConfigured)
	cfg.SetDefaults()
	cfg.NormalizeNilFields()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	warnCompatibilityDeprecation(cfg.ResolveCompatibilityContext())
	return &cfg, nil
}

func (c *Config) applyPostLoadNormalization(matchBuildModelConfigured bool) {
	c.Project.Profile = strings.ToLower(strings.TrimSpace(c.Project.Profile))
	c.Tracker.Backend = strings.ToLower(strings.TrimSpace(c.Tracker.Backend))
	c.Methodology.Adapter = strings.ToLower(strings.TrimSpace(c.Methodology.Adapter))
	c.Review.Tier = normalizeConfiguredTier(c.Review.Tier)
	c.Review.Thorough.Tier = normalizeConfiguredTier(c.Review.Thorough.Tier)
	c.TokenEfficiency.Routing.UtilityTier = normalizeConfiguredTier(c.TokenEfficiency.Routing.UtilityTier)
	c.Methodology.BuildStrategy = strings.ToLower(strings.TrimSpace(c.Methodology.BuildStrategy))
	c.Methodology.PhaseModels.Decompose = normalizeConfiguredTier(c.Methodology.PhaseModels.Decompose)
	c.Methodology.PhaseModels.Build = normalizeConfiguredTier(c.Methodology.PhaseModels.Build)
	c.Methodology.PhaseModels.Red = normalizeConfiguredTier(c.Methodology.PhaseModels.Red)
	c.Methodology.PhaseModels.Green = normalizeConfiguredTier(c.Methodology.PhaseModels.Green)
	c.Methodology.PhaseModels.Refactor = normalizeConfiguredTier(c.Methodology.PhaseModels.Refactor)
	if len(c.TokenEfficiency.Routing.TaskOverrides) > 0 {
		normalizedOverrides := make(map[string]string, len(c.TokenEfficiency.Routing.TaskOverrides))
		for category, tier := range c.TokenEfficiency.Routing.TaskOverrides {
			normalizedCategory := strings.ToLower(strings.TrimSpace(category))
			normalizedOverrides[normalizedCategory] = normalizeConfiguredTier(tier)
		}
		c.TokenEfficiency.Routing.TaskOverrides = normalizedOverrides
	}

	if c.Review.Tier == "" {
		if normalizedTier := normalizedLegacyModelTier(c.Review.Model); normalizedTier != "" {
			c.Review.Tier = normalizedTier
		}
	}
	if c.Review.Thorough.Tier == "" {
		if normalizedTier := normalizedLegacyModelTier(c.Review.Thorough.Model); normalizedTier != "" {
			c.Review.Thorough.Tier = normalizedTier
		}
	}
	if matchBuildModelConfigured {
		warnConfigDeprecation("review.match_build_model is deprecated, ignored, and will be removed in a future release")
	}
}

func normalizedLegacyModelTier(model string) string {
	normalizedModel := strings.TrimSpace(model)
	if normalizedModel == "" {
		return ""
	}
	tier := tierFromLegacyModel(normalizedModel)
	if tier == normalizedModel {
		return ""
	}
	return tier
}

func normalizeConfiguredTier(tier string) string {
	return strings.ToLower(strings.TrimSpace(tier))
}

func warnConfigDeprecation(message string) {
	_, _ = fmt.Fprintf(configWarningWriter, "Warning: %s\n", message)
}

func warnCompatibilityDeprecation(ctx CompatibilityContext) {
	markers := CompatibilityDeprecationSummary(ctx)
	if markers == "" {
		return
	}

	warnConfigDeprecation(
		fmt.Sprintf(
			"%s active; set compatibility.strict_legacy_fallback: true now (strict-by-default planned after %s)",
			markers,
			CompatibilityStrictDefaultCutoverDate,
		),
	)
}

// Validate ensures config values are within supported ranges.
func (c *Config) Validate() error {
	if c.Validation.PlanMaxSubBeads != nil && *c.Validation.PlanMaxSubBeads < 0 {
		return fmt.Errorf("validation.plan_max_sub_beads must be >= 0 (got %d)", *c.Validation.PlanMaxSubBeads)
	}
	if c.Validation.RuntimeMaxSubBeadsValue() <= 0 {
		return fmt.Errorf("validation.runtime_max_sub_beads must be > 0 (got %d)", c.Validation.RuntimeMaxSubBeads)
	}
	if err := c.validateCompatibilitySelections(); err != nil {
		return err
	}
	if err := c.validateCompatibilityPolicy(); err != nil {
		return err
	}
	if err := c.Methodology.Validate(); err != nil {
		return err
	}
	if err := c.Routing.CircuitBreaker.Validate(); err != nil {
		return err
	}
	if err := c.validateTokenEfficiencyRouting(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateTokenEfficiencyRouting() error {
	validTiers := map[string]struct{}{
		"low":    {},
		"medium": {},
		"high":   {},
	}
	validUtilityCategories := map[string]struct{}{
		"summarization":      {},
		"masking_transform":  {},
		"discovery_indexing": {},
	}

	utilityTier := strings.TrimSpace(c.TokenEfficiency.Routing.UtilityTier)
	if utilityTier != "" {
		if _, ok := validTiers[utilityTier]; !ok {
			return fmt.Errorf(
				"token_efficiency.routing.utility_tier must be one of [low medium high] (got %q)",
				c.TokenEfficiency.Routing.UtilityTier,
			)
		}
	}

	for category, tier := range c.TokenEfficiency.Routing.TaskOverrides {
		normalizedCategory := strings.ToLower(strings.TrimSpace(category))
		if _, ok := validUtilityCategories[normalizedCategory]; !ok {
			return fmt.Errorf(
				"token_efficiency.routing.task_overrides contains unsupported category %q (allowed: summarization, masking_transform, discovery_indexing)",
				category,
			)
		}
		normalizedTier := strings.ToLower(strings.TrimSpace(tier))
		if _, ok := validTiers[normalizedTier]; !ok {
			return fmt.Errorf(
				"token_efficiency.routing.task_overrides[%q] must be one of [low medium high] (got %q)",
				category,
				tier,
			)
		}
	}
	return nil
}

func (c *Config) validateCompatibilitySelections() error {
	if c.Project.Profile != "" && c.Project.Profile != "go" {
		return fmt.Errorf("project.profile must be %q (got %q)", "go", c.Project.Profile)
	}
	if c.Tracker.Backend != "" && c.Tracker.Backend != "bd" {
		return fmt.Errorf("tracker.backend must be %q (got %q)", "bd", c.Tracker.Backend)
	}
	if c.Methodology.Adapter != "" && c.Methodology.Adapter != "go" {
		return fmt.Errorf("methodology.adapter must be %q (got %q)", "go", c.Methodology.Adapter)
	}
	return nil
}

func (c *Config) validateCompatibilityPolicy() error {
	if !c.Compatibility.StrictLegacyFallback {
		return nil
	}

	resolved := c.ResolveCompatibilityContext()
	if resolved.Profile.Source == CompatibilitySourceLegacyFallback ||
		resolved.TrackerBackend.Source == CompatibilitySourceLegacyFallback ||
		resolved.MethodologyAdapter.Source == CompatibilitySourceLegacyFallback {
		return fmt.Errorf("compatibility.strict_legacy_fallback requires explicit selectors (project.profile, tracker.backend, methodology.adapter)")
	}

	return nil
}

// Validate ensures methodology settings are supported.
func (m MethodologyConfig) Validate() error {
	switch m.Granularity {
	case MethodologyGranularityBead, MethodologyGranularitySpec:
		return nil
	default:
		return fmt.Errorf(
			"methodology.granularity must be %q or %q (got %q)",
			MethodologyGranularityBead,
			MethodologyGranularitySpec,
			m.Granularity,
		)
	}
}

func (c CircuitBreakerConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.WindowSize <= 0 {
		return fmt.Errorf("routing.circuit_breaker.window_size must be > 0 (got %d)", c.WindowSize)
	}
	if c.FailureThreshold <= 0 || c.FailureThreshold > 1 {
		return fmt.Errorf("routing.circuit_breaker.failure_threshold must be > 0 and <= 1 (got %v)", c.FailureThreshold)
	}
	if c.DegradedFloor <= 0 || c.DegradedFloor > 100 {
		return fmt.Errorf("routing.circuit_breaker.degraded_floor must be > 0 and <= 100 (got %d)", c.DegradedFloor)
	}
	if c.RecoverySuccesses <= 0 {
		return fmt.Errorf("routing.circuit_breaker.recovery_successes must be > 0 (got %d)", c.RecoverySuccesses)
	}

	return nil
}

// ScopeGoTestCommands scopes "go test ./..." commands to touched packages.
// Non-go-test commands and commands without "./..." are returned unchanged.
func ScopeGoTestCommands(commands []string, touchedPackages []string) []string {
	const (
		goCommand       = "go"
		testCommand     = "test"
		allPackagesExpr = "./..."
	)

	if len(commands) == 0 || len(touchedPackages) == 0 {
		return commands
	}

	uniqueTouched := normalizeTouchedPackages(touchedPackages)
	collapsed := collapsePackageScopes(uniqueTouched)

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
		if len(fields) < 3 || fields[0] != goCommand || fields[1] != testCommand {
			scoped = append(scoped, command)
			continue
		}

		replaced := false
		rebuilt := make([]string, 0, len(fields)+len(scopedPackages))
		for _, token := range fields {
			if token == allPackagesExpr {
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

func normalizeTouchedPackages(touchedPackages []string) []string {
	uniqueTouched := make([]string, 0, len(touchedPackages))
	seen := make(map[string]struct{}, len(touchedPackages))

	for _, pkg := range touchedPackages {
		trimmed := strings.TrimSpace(pkg)
		normalizedPkg := strings.Trim(strings.TrimPrefix(trimmed, "./"), "/")
		if trimmed == "." || normalizedPkg == "." {
			normalizedPkg = "."
		}

		if normalizedPkg == "" {
			continue
		}

		if _, exists := seen[normalizedPkg]; exists {
			continue
		}

		seen[normalizedPkg] = struct{}{}
		uniqueTouched = append(uniqueTouched, normalizedPkg)
	}

	return uniqueTouched
}

// collapsePackageScopes removes child packages when a parent package already
// covers them via `/...`.
func collapsePackageScopes(packages []string) []string {
	collapsed := make([]string, 0, len(packages))

	for _, pkg := range packages {
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

		// Keep a separate backing array to avoid mutating entries while iterating collapsed.
		filtered := make([]string, 0, len(collapsed))
		for _, existing := range collapsed {
			if existing == pkg || strings.HasPrefix(existing, pkg+"/") {
				continue
			}
			filtered = append(filtered, existing)
		}
		collapsed = append(filtered, pkg)
	}

	return collapsed
}
