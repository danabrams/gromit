package config

import "strings"

type CompatibilitySource string

const (
	CompatibilitySourceExplicit       CompatibilitySource = "explicit"
	CompatibilitySourceProfileDefault CompatibilitySource = "profile_default"
	CompatibilitySourceLegacyFallback CompatibilitySource = "legacy_fallback"

	// CompatibilityDeprecationMarkerLegacyHardcodedDefaults marks values resolved by
	// legacy hard-coded compatibility shims that should be removed after migration.
	CompatibilityDeprecationMarkerLegacyHardcodedDefaults = "compat-deprecated-legacy-hardcoded-defaults"
	CompatibilityStrictDefaultCutoverDate                = "2026-06-01"
)

type CompatibilityResolvedValue struct {
	Value             string
	Source            CompatibilitySource
	DeprecationMarker string
}

type CompatibilityContext struct {
	Profile           CompatibilityResolvedValue
	TrackerBackend    CompatibilityResolvedValue
	MethodologyAdapter CompatibilityResolvedValue
}

func (c Config) ResolveCompatibilityContext() CompatibilityContext {
	profile := CompatibilityResolvedValue{
		Value:             "go",
		Source:            CompatibilitySourceLegacyFallback,
		DeprecationMarker: CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
	}
	if c.Project.Profile != "" {
		profile = CompatibilityResolvedValue{
			Value:  c.Project.Profile,
			Source: CompatibilitySourceExplicit,
		}
	}

	backend := CompatibilityResolvedValue{
		Value:             "bd",
		Source:            CompatibilitySourceLegacyFallback,
		DeprecationMarker: CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
	}
	adapter := CompatibilityResolvedValue{
		Value:             "go",
		Source:            CompatibilitySourceLegacyFallback,
		DeprecationMarker: CompatibilityDeprecationMarkerLegacyHardcodedDefaults,
	}
	if profile.Source == CompatibilitySourceExplicit {
		backend.Source = CompatibilitySourceProfileDefault
		adapter.Source = CompatibilitySourceProfileDefault
	}
	if c.Tracker.Backend != "" {
		backend = CompatibilityResolvedValue{
			Value:  c.Tracker.Backend,
			Source: CompatibilitySourceExplicit,
		}
	}
	if c.Methodology.Adapter != "" {
		adapter = CompatibilityResolvedValue{
			Value:  c.Methodology.Adapter,
			Source: CompatibilitySourceExplicit,
		}
	}

	return CompatibilityContext{
		Profile:            profile,
		TrackerBackend:     backend,
		MethodologyAdapter: adapter,
	}
}

func CompatibilityDeprecationMarkers(ctx CompatibilityContext) []string {
	markers := make([]string, 0, 3)
	seen := map[string]struct{}{}

	for _, marker := range []string{
		ctx.Profile.DeprecationMarker,
		ctx.TrackerBackend.DeprecationMarker,
		ctx.MethodologyAdapter.DeprecationMarker,
	} {
		if marker == "" {
			continue
		}
		if _, exists := seen[marker]; exists {
			continue
		}
		seen[marker] = struct{}{}
		markers = append(markers, marker)
	}

	return markers
}

func CompatibilityDeprecationSummary(ctx CompatibilityContext) string {
	markers := CompatibilityDeprecationMarkers(ctx)
	if len(markers) == 0 {
		return ""
	}
	return strings.Join(markers, ", ")
}
