package logger

import "testing"

func TestResolveProviderNameVariants(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		model    string
		want     string
	}{
		{name: "explicit provider", provider: "anthropic", model: "", want: "anthropic"},
		{name: "openai-inferred", provider: "", model: "GPT-4o-mini", want: "openai"},
		{name: "claude-inferred", provider: "", model: "claude-2", want: "claude"},
		{name: "unknown when blank", provider: "", model: "", want: "unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveProviderName(tc.provider, tc.model); got != tc.want {
				t.Fatalf("resolveProviderName(%q, %q) = %q, want %q", tc.provider, tc.model, got, tc.want)
			}
		})
	}
}
