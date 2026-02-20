package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

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
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	return &cfg, nil
}

// Validate ensures config values are within supported ranges.
func (c *Config) Validate() error {
	if err := c.Methodology.Validate(); err != nil {
		return err
	}
	return nil
}

// Validate ensures methodology settings are supported.
func (m MethodologyConfig) Validate() error {
	switch strings.ToLower(m.Granularity) {
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
		normalized := strings.Trim(strings.TrimPrefix(trimmed, "./"), "/")
		if trimmed == "." || normalized == "." {
			normalized = "."
		}

		if _, exists := seen[normalized]; normalized == "" || exists {
			continue
		}

		seen[normalized] = struct{}{}
		uniqueTouched = append(uniqueTouched, normalized)
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
