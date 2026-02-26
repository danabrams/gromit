package config

import "strings"

var defaultGeminiTierToModelMap = map[string]string{
	"high":   "gemini-3.1-pro",
	"medium": "gemini-3-flash",
	"low":    "gemini-3-flash",
}

// ResolveGeminiModelMap returns the tier-to-model mappings for the Gemini provider.
// Provider-level overrides only replace the defaults that are provided.
func ResolveGeminiModelMap(def ProviderDef) map[string]string {
	resolved := cloneGeminiModelMap(defaultGeminiTierToModelMap)
	for tier, model := range def.Models {
		if strings.TrimSpace(model) == "" {
			continue
		}
		resolved[tier] = model
	}
	return resolved
}

func cloneGeminiModelMap(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for tier, model := range src {
		dst[tier] = model
	}
	return dst
}
