package config

import (
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

// TestSelectTierBasics verifies that SelectTier() method exists on Config
// and returns tier names based on priority with defaults applied.
// Expected failure: SelectTier() method does not exist on Config yet
func TestSelectTierBasics(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		priority int
		labels   []string
		want     string
	}{
		{
			name:     "NilReceiver",
			cfg:      nil,
			priority: 1,
			labels:   nil,
			want:     provider.TierMedium,
		},
		{
			name: "P0ReturnsTierHigh",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
			}},
			priority: 0,
			labels:   nil,
			want:     provider.TierHigh,
		},
		{
			name: "P1ReturnsTierMedium",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
			}},
			priority: 1,
			labels:   nil,
			want:     provider.TierMedium,
		},
		{
			name: "P2ReturnsTierLow",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
			}},
			priority: 2,
			labels:   nil,
			want:     provider.TierLow,
		},
		{
			name: "UnknownPriorityDefaultsToMedium",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
			}},
			priority: 99,
			labels:   nil,
			want:     provider.TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *Config
			if tt.cfg != nil {
				cfg = tt.cfg
			}
			result := cfg.SelectTier(tt.priority, tt.labels)
			if result != tt.want {
				t.Errorf("SelectTier(%d, %v) = %q, want %q", tt.priority, tt.labels, result, tt.want)
			}
		})
	}
}

// TestSelectTierLabelOverrides verifies that SelectTier() respects label overrides
// with higher precedence than priority-based selection.
// Expected failure: SelectTier() method does not exist on Config yet
func TestSelectTierLabelOverrides(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		priority int
		labels   []string
		want     string
	}{
		{
			name: "ComplexityHighLabelOverridesP1",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
				Labels: map[string]string{
					"complexity:high": provider.TierHigh,
				},
			}},
			priority: 1, // Would normally return medium
			labels:   []string{"complexity:high"},
			want:     provider.TierHigh,
		},
		{
			name: "ComplexityLowLabelOverridesP0",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
				Labels: map[string]string{
					"complexity:low": provider.TierLow,
				},
			}},
			priority: 0, // Would normally return high
			labels:   []string{"complexity:low"},
			want:     provider.TierLow,
		},
		{
			name: "FirstMatchingLabelWins",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
				Labels: map[string]string{
					"complexity:high": provider.TierHigh,
					"complexity:low":  provider.TierLow,
				},
			}},
			priority: 1,
			labels:   []string{"complexity:high", "complexity:low"},
			want:     provider.TierHigh, // First label in slice wins
		},
		{
			name: "UnmatchedLabelsFallbackToPriority",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
				Labels: map[string]string{
					"complexity:high": provider.TierHigh,
				},
			}},
			priority: 2,
			labels:   []string{"spec:example"}, // Not in Labels map
			want:     provider.TierLow,          // Falls back to P2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.SelectTier(tt.priority, tt.labels)
			if result != tt.want {
				t.Errorf("SelectTier(%d, %v) = %q, want %q", tt.priority, tt.labels, result, tt.want)
			}
		})
	}
}

// TestSelectTierBackwardCompatibility verifies that SelectTier() provides backward
// compatibility by auto-mapping legacy model names via TierFromLegacyModel when
// Models.P0/P1/P2 contain known model names instead of tier names.
// Expected failure: SelectTier() method does not exist, and backward compatibility logic is not implemented
func TestSelectTierBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		priority int
		labels   []string
		want     string
	}{
		{
			name: "P0OpusAutoMapsToTierHigh",
			cfg: &Config{Models: ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			}},
			priority: 0,
			labels:   nil,
			want:     provider.TierHigh,
		},
		{
			name: "P1SonnetAutoMapsToTierMedium",
			cfg: &Config{Models: ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			}},
			priority: 1,
			labels:   nil,
			want:     provider.TierMedium,
		},
		{
			name: "P2HaikuAutoMapsToTierLow",
			cfg: &Config{Models: ModelsConfig{
				P0: "opus",
				P1: "sonnet",
				P2: "haiku",
			}},
			priority: 2,
			labels:   nil,
			want:     provider.TierLow,
		},
		{
			name: "LabelOverrideOpusAutoMapsToTierHigh",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
				Labels: map[string]string{
					"complexity:high": "opus", // Legacy model name
				},
			}},
			priority: 1,
			labels:   []string{"complexity:high"},
			want:     provider.TierHigh,
		},
		{
			name: "LabelOverrideHaikuAutoMapsToTierLow",
			cfg: &Config{Models: ModelsConfig{
				P0: provider.TierHigh,
				P1: provider.TierMedium,
				P2: provider.TierLow,
				Labels: map[string]string{
					"complexity:low": "haiku", // Legacy model name
				},
			}},
			priority: 0,
			labels:   []string{"complexity:low"},
			want:     provider.TierLow,
		},
		{
			name: "OpenAIGPT4oAutoMapsToTierMedium",
			cfg: &Config{Models: ModelsConfig{
				P0: "o3",
				P1: "gpt-4o",
				P2: "gpt-4o-mini",
			}},
			priority: 1,
			labels:   nil,
			want:     provider.TierMedium,
		},
		{
			name: "MixedTierAndLegacyConfig",
			cfg: &Config{Models: ModelsConfig{
				P0: "opus",               // Legacy
				P1: provider.TierMedium,  // Tier
				P2: "haiku",              // Legacy
			}},
			priority: 0,
			labels:   nil,
			want:     provider.TierHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.SelectTier(tt.priority, tt.labels)
			if result != tt.want {
				t.Errorf("SelectTier(%d, %v) = %q, want %q", tt.priority, tt.labels, result, tt.want)
			}
		})
	}
}

// TestIsTierNameBasics verifies that IsTierName() method exists on Config
// and correctly identifies tier names versus legacy model names.
// Expected failure: IsTierName() method does not exist on Config yet
func TestIsTierNameBasics(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "HighIsTier",
			input: provider.TierHigh,
			want:  true,
		},
		{
			name:  "MediumIsTier",
			input: provider.TierMedium,
			want:  true,
		},
		{
			name:  "LowIsTier",
			input: provider.TierLow,
			want:  true,
		},
		{
			name:  "OpusIsNotTier",
			input: "opus",
			want:  false,
		},
		{
			name:  "SonnetIsNotTier",
			input: "sonnet",
			want:  false,
		},
		{
			name:  "HaikuIsNotTier",
			input: "haiku",
			want:  false,
		},
		{
			name:  "O3IsNotTier",
			input: "o3",
			want:  false,
		},
		{
			name:  "GPT4oIsNotTier",
			input: "gpt-4o",
			want:  false,
		},
		{
			name:  "GPT4oMiniIsNotTier",
			input: "gpt-4o-mini",
			want:  false,
		},
		{
			name:  "EmptyStringIsNotTier",
			input: "",
			want:  false,
		},
		{
			name:  "UnknownStringIsNotTier",
			input: "future-model",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			result := cfg.IsTierName(tt.input)
			if result != tt.want {
				t.Errorf("IsTierName(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

// TestIsTierNameCaseInsensitive verifies that IsTierName() handles case variations.
// Expected failure: IsTierName() method does not exist on Config yet
func TestIsTierNameCaseInsensitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "HighUppercase",
			input: "HIGH",
			want:  true,
		},
		{
			name:  "MediumMixedCase",
			input: "Medium",
			want:  true,
		},
		{
			name:  "LowUppercase",
			input: "LOW",
			want:  true,
		},
		{
			name:  "OpusUppercase",
			input: "OPUS",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			result := cfg.IsTierName(tt.input)
			if result != tt.want {
				t.Errorf("IsTierName(%q) = %v, want %v", tt.input, result, tt.want)
			}
		})
	}
}

// TestIsTierNameNilReceiver verifies that IsTierName() handles nil receiver gracefully.
// Expected failure: IsTierName() method does not exist on Config yet
func TestIsTierNameNilReceiver(t *testing.T) {
	var cfg *Config
	result := cfg.IsTierName(provider.TierHigh)
	if !result {
		t.Errorf("IsTierName(%q) on nil receiver = %v, want true", provider.TierHigh, result)
	}
}

// TestNextEscalationTierBasics verifies that NextEscalationTier() method exists on Config
// and returns the next tier in the escalation chain using abstract tier names.
// Expected failure: NextEscalationTier() method does not exist on Config yet
func TestNextEscalationTierBasics(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		currentTier string
		want        string
	}{
		{
			name:        "NilReceiver",
			cfg:         nil,
			currentTier: provider.TierLow,
			want:        "",
		},
		{
			name: "EscalationDisabled",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: false,
				Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh},
			}},
			currentTier: provider.TierLow,
			want:        "",
		},
		{
			name: "LowToMedium",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh},
			}},
			currentTier: provider.TierLow,
			want:        provider.TierMedium,
		},
		{
			name: "MediumToHigh",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh},
			}},
			currentTier: provider.TierMedium,
			want:        provider.TierHigh,
		},
		{
			name: "HighIsEndOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh},
			}},
			currentTier: provider.TierHigh,
			want:        "",
		},
		{
			name: "NotInChainReturnsEmpty",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh},
			}},
			currentTier: "unknown-tier",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *Config
			if tt.cfg != nil {
				cfg = tt.cfg
			}
			result := cfg.NextEscalationTier(tt.currentTier)
			if result != tt.want {
				t.Errorf("NextEscalationTier(%q) = %q, want %q", tt.currentTier, result, tt.want)
			}
		})
	}
}

// TestNextEscalationTierBackwardCompatibility verifies that NextEscalationTier()
// handles legacy model names in the chain by auto-mapping them to tiers.
// Expected failure: NextEscalationTier() method does not exist, and backward compatibility logic is not implemented
func TestNextEscalationTierBackwardCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		currentTier string
		want        string
	}{
		{
			name: "LegacyHaikuToSonnetInChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"}, // Legacy names
			}},
			currentTier: provider.TierLow, // Tier-based input
			want:        provider.TierMedium,
		},
		{
			name: "LegacySonnetToOpusInChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"}, // Legacy names
			}},
			currentTier: provider.TierMedium, // Tier-based input
			want:        provider.TierHigh,
		},
		{
			name: "LegacyOpusIsEndOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"}, // Legacy names
			}},
			currentTier: provider.TierHigh, // Tier-based input
			want:        "",
		},
		{
			name: "LegacyInputHaikuToSonnet",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh}, // Tier names
			}},
			currentTier: "haiku", // Legacy input
			want:        provider.TierMedium,
		},
		{
			name: "MixedChainLegacyAndTiers",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", provider.TierMedium, "opus"}, // Mixed
			}},
			currentTier: provider.TierLow,
			want:        provider.TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cfg.NextEscalationTier(tt.currentTier)
			if result != tt.want {
				t.Errorf("NextEscalationTier(%q) with chain %v = %q, want %q",
					tt.currentTier, tt.cfg.Escalation.Chain, result, tt.want)
			}
		})
	}
}

// TestNextEscalationTierEmptyChain verifies that NextEscalationTier() returns
// empty string when the escalation chain is empty.
// Expected failure: NextEscalationTier() method does not exist on Config yet
func TestNextEscalationTierEmptyChain(t *testing.T) {
	cfg := &Config{Escalation: EscalationConfig{
		Enabled: true,
		Chain:   []string{},
	}}

	result := cfg.NextEscalationTier(provider.TierLow)
	if result != "" {
		t.Errorf("NextEscalationTier(%q) with empty chain = %q, want empty", provider.TierLow, result)
	}
}

// TestSelectTierAndNextEscalationTierIntegration verifies that SelectTier()
// and NextEscalationTier() work together correctly for the full escalation flow.
// Expected failure: SelectTier() and NextEscalationTier() methods do not exist on Config yet
func TestSelectTierAndNextEscalationTierIntegration(t *testing.T) {
	cfg := &Config{
		Models: ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{provider.TierLow, provider.TierMedium, provider.TierHigh},
		},
	}

	// Start with P2 bead
	initialTier := cfg.SelectTier(2, nil)
	if initialTier != provider.TierLow {
		t.Errorf("SelectTier(2, nil) = %q, want %q", initialTier, provider.TierLow)
	}

	// First escalation
	nextTier := cfg.NextEscalationTier(initialTier)
	if nextTier != provider.TierMedium {
		t.Errorf("NextEscalationTier(%q) = %q, want %q", initialTier, nextTier, provider.TierMedium)
	}

	// Second escalation
	finalTier := cfg.NextEscalationTier(nextTier)
	if finalTier != provider.TierHigh {
		t.Errorf("NextEscalationTier(%q) = %q, want %q", nextTier, finalTier, provider.TierHigh)
	}

	// End of chain
	endOfChain := cfg.NextEscalationTier(finalTier)
	if endOfChain != "" {
		t.Errorf("NextEscalationTier(%q) = %q, want empty", finalTier, endOfChain)
	}
}

// TestSelectTierWithDefaultsApplied verifies that SelectTier() works correctly
// when config defaults are applied via SetDefaults().
// Expected failure: SelectTier() method does not exist on Config yet
func TestSelectTierWithDefaultsApplied(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// After SetDefaults(), Models.P0 should be "opus" (legacy model)
	// SelectTier should auto-map it to TierHigh
	result := cfg.SelectTier(0, nil)
	if result != provider.TierHigh {
		t.Errorf("SelectTier(0, nil) after SetDefaults() = %q, want %q", result, provider.TierHigh)
	}

	result = cfg.SelectTier(1, nil)
	if result != provider.TierMedium {
		t.Errorf("SelectTier(1, nil) after SetDefaults() = %q, want %q", result, provider.TierMedium)
	}

	result = cfg.SelectTier(2, nil)
	if result != provider.TierLow {
		t.Errorf("SelectTier(2, nil) after SetDefaults() = %q, want %q", result, provider.TierLow)
	}
}
