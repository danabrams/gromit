package config

type CompatibilitySource string

const (
	CompatibilitySourceExplicit       CompatibilitySource = "explicit"
	CompatibilitySourceProfileDefault CompatibilitySource = "profile_default"
	CompatibilitySourceLegacyFallback CompatibilitySource = "legacy_fallback"
)

type CompatibilityResolvedValue struct {
	Value  string
	Source CompatibilitySource
}

type CompatibilityContext struct {
	Profile           CompatibilityResolvedValue
	TrackerBackend    CompatibilityResolvedValue
	MethodologyAdapter CompatibilityResolvedValue
}

func (c Config) ResolveCompatibilityContext() CompatibilityContext {
	profile := CompatibilityResolvedValue{
		Value:  "go",
		Source: CompatibilitySourceLegacyFallback,
	}
	if c.Project.Profile != "" {
		profile = CompatibilityResolvedValue{
			Value:  c.Project.Profile,
			Source: CompatibilitySourceExplicit,
		}
	}

	backend := CompatibilityResolvedValue{
		Value:  "bd",
		Source: CompatibilitySourceLegacyFallback,
	}
	adapter := CompatibilityResolvedValue{
		Value:  "go",
		Source: CompatibilitySourceLegacyFallback,
	}
	if profile.Source == CompatibilitySourceExplicit {
		backend.Source = CompatibilitySourceProfileDefault
		adapter.Source = CompatibilitySourceProfileDefault
	}

	return CompatibilityContext{
		Profile:            profile,
		TrackerBackend:     backend,
		MethodologyAdapter: adapter,
	}
}
