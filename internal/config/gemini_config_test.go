package config

import "testing"

func TestResolveGeminiModelMap(t *testing.T) {
	t.Run("uses provider model overrides", func(t *testing.T) {
		def := ProviderDef{
			Models: map[string]string{
				"high": "gemini-custom-high",
			},
		}

		got := ResolveGeminiModelMap(def)
		if got["high"] != "gemini-custom-high" {
			t.Fatalf("expected custom high model, got %q", got["high"])
		}
	})

	t.Run("falls back to defaults", func(t *testing.T) {
		def := ProviderDef{}

		got := ResolveGeminiModelMap(def)
		if got["high"] != "gemini-3.1-pro" {
			t.Fatalf("expected default high model, got %q", got["high"])
		}
		if got["medium"] != "gemini-3-flash" {
			t.Fatalf("expected default medium model, got %q", got["medium"])
		}
		if got["low"] != "gemini-3-flash" {
			t.Fatalf("expected default low model, got %q", got["low"])
		}
	})
}
