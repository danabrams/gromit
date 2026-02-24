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
	return CompatibilityContext{
		Profile: CompatibilityResolvedValue{
			Value:  "go",
			Source: CompatibilitySourceLegacyFallback,
		},
		TrackerBackend: CompatibilityResolvedValue{
			Value:  "bd",
			Source: CompatibilitySourceLegacyFallback,
		},
		MethodologyAdapter: CompatibilityResolvedValue{
			Value:  "go",
			Source: CompatibilitySourceLegacyFallback,
		},
	}
}
