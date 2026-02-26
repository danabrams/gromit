package config

var defaultGeminiTierToModelMap = map[string]string{
	"high":   "gemini-3.1-pro",
	"medium": "gemini-3-flash",
	"low":    "gemini-3-flash",
}

// ResolveGeminiModelMap returns the tier-to-model mappings for the Gemini provider.
// Provider-level overrides take precedence over the built-in defaults.
func ResolveGeminiModelMap(def ProviderDef) map[string]string {
	if len(def.Models) > 0 {
		return cloneGeminiModelMap(def.Models)
	}
	return cloneGeminiModelMap(defaultGeminiTierToModelMap)
}

func cloneGeminiModelMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for tier, model := range src {
		dst[tier] = model
	}
	return dst
}
