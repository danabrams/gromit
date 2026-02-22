package config

import (
	"testing"
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
			want:     "medium",
		},
		{
			name: "P0ReturnsTierHigh",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
			}},
			priority: 0,
			labels:   nil,
			want:     "high",
		},
		{
			name: "P1ReturnsTierMedium",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
			}},
			priority: 1,
			labels:   nil,
			want:     "medium",
		},
		{
			name: "P2ReturnsTierLow",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
			}},
			priority: 2,
			labels:   nil,
			want:     "low",
		},
		{
			name: "UnknownPriorityDefaultsToMedium",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
			}},
			priority: 99,
			labels:   nil,
			want:     "medium",
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
				P0: "high",
				P1: "medium",
				P2: "low",
				Labels: map[string]string{
					"complexity:high": "high",
				},
			}},
			priority: 1, // Would normally return medium
			labels:   []string{"complexity:high"},
			want:     "high",
		},
		{
			name: "ComplexityLowLabelOverridesP0",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
				Labels: map[string]string{
					"complexity:low": "low",
				},
			}},
			priority: 0, // Would normally return high
			labels:   []string{"complexity:low"},
			want:     "low",
		},
		{
			name: "FirstMatchingLabelWins",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
				Labels: map[string]string{
					"complexity:high": "high",
					"complexity:low":  "low",
				},
			}},
			priority: 1,
			labels:   []string{"complexity:high", "complexity:low"},
			want:     "high", // First label in slice wins
		},
		{
			name: "UnmatchedLabelsFallbackToPriority",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
				Labels: map[string]string{
					"complexity:high": "high",
				},
			}},
			priority: 2,
			labels:   []string{"spec:example"}, // Not in Labels map
			want:     "low",                    // Falls back to P2
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
			want:     "high",
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
			want:     "medium",
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
			want:     "low",
		},
		{
			name: "LabelOverrideOpusAutoMapsToTierHigh",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
				Labels: map[string]string{
					"complexity:high": "opus", // Legacy model name
				},
			}},
			priority: 1,
			labels:   []string{"complexity:high"},
			want:     "high",
		},
		{
			name: "LabelOverrideHaikuAutoMapsToTierLow",
			cfg: &Config{Models: ModelsConfig{
				P0: "high",
				P1: "medium",
				P2: "low",
				Labels: map[string]string{
					"complexity:low": "haiku", // Legacy model name
				},
			}},
			priority: 0,
			labels:   []string{"complexity:low"},
			want:     "low",
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
			want:     "medium",
		},
		{
			name: "MixedTierAndLegacyConfig",
			cfg: &Config{Models: ModelsConfig{
				P0: "opus",   // Legacy
				P1: "medium", // Tier
				P2: "haiku",  // Legacy
			}},
			priority: 0,
			labels:   nil,
			want:     "high",
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
			input: "high",
			want:  true,
		},
		{
			name:  "MediumIsTier",
			input: "medium",
			want:  true,
		},
		{
			name:  "LowIsTier",
			input: "low",
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
	result := cfg.IsTierName("high")
	if !result {
		t.Errorf("IsTierName(%q) on nil receiver = %v, want true", "high", result)
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
			currentTier: "low",
			want:        "",
		},
		{
			name: "EscalationDisabled",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: false,
				Chain:   []string{"low", "medium", "high"},
			}},
			currentTier: "low",
			want:        "",
		},
		{
			name: "LowToMedium",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"low", "medium", "high"},
			}},
			currentTier: "low",
			want:        "medium",
		},
		{
			name: "MediumToHigh",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"low", "medium", "high"},
			}},
			currentTier: "medium",
			want:        "high",
		},
		{
			name: "HighIsEndOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"low", "medium", "high"},
			}},
			currentTier: "high",
			want:        "",
		},
		{
			name: "NotInChainReturnsEmpty",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"low", "medium", "high"},
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
			currentTier: "low", // Tier-based input
			want:        "medium",
		},
		{
			name: "LegacySonnetToOpusInChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"}, // Legacy names
			}},
			currentTier: "medium", // Tier-based input
			want:        "high",
		},
		{
			name: "LegacyOpusIsEndOfChain",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "sonnet", "opus"}, // Legacy names
			}},
			currentTier: "high", // Tier-based input
			want:        "",
		},
		{
			name: "LegacyInputHaikuToSonnet",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"low", "medium", "high"}, // Tier names
			}},
			currentTier: "haiku", // Legacy input
			want:        "medium",
		},
		{
			name: "MixedChainLegacyAndTiers",
			cfg: &Config{Escalation: EscalationConfig{
				Enabled: true,
				Chain:   []string{"haiku", "medium", "opus"}, // Mixed
			}},
			currentTier: "low",
			want:        "medium",
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

	result := cfg.NextEscalationTier("low")
	if result != "" {
		t.Errorf("NextEscalationTier(%q) with empty chain = %q, want empty", "low", result)
	}
}

// TestSelectTierAndNextEscalationTierIntegration verifies that SelectTier()
// and NextEscalationTier() work together correctly for the full escalation flow.
// Expected failure: SelectTier() and NextEscalationTier() methods do not exist on Config yet
func TestSelectTierAndNextEscalationTierIntegration(t *testing.T) {
	cfg := &Config{
		Models: ModelsConfig{
			P0: "high",
			P1: "medium",
			P2: "low",
		},
		Escalation: EscalationConfig{
			Enabled: true,
			Chain:   []string{"low", "medium", "high"},
		},
	}

	// Start with P2 bead
	initialTier := cfg.SelectTier(2, nil)
	if initialTier != "low" {
		t.Errorf("SelectTier(2, nil) = %q, want %q", initialTier, "low")
	}

	// First escalation
	nextTier := cfg.NextEscalationTier(initialTier)
	if nextTier != "medium" {
		t.Errorf("NextEscalationTier(%q) = %q, want %q", initialTier, nextTier, "medium")
	}

	// Second escalation
	finalTier := cfg.NextEscalationTier(nextTier)
	if finalTier != "high" {
		t.Errorf("NextEscalationTier(%q) = %q, want %q", nextTier, finalTier, "high")
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
	if result != "high" {
		t.Errorf("SelectTier(0, nil) after SetDefaults() = %q, want %q", result, "high")
	}

	result = cfg.SelectTier(1, nil)
	if result != "medium" {
		t.Errorf("SelectTier(1, nil) after SetDefaults() = %q, want %q", result, "medium")
	}

	result = cfg.SelectTier(2, nil)
	if result != "low" {
		t.Errorf("SelectTier(2, nil) after SetDefaults() = %q, want %q", result, "low")
	}
}
