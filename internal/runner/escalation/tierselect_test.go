package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// --- Tier/model selection tests ---

// Expected failure: SelectTier function does not exist in escalation/ package yet
func TestSelectTier_DelegatesToConfig(t *testing.T) {
	// SelectTier should delegate to config.Config.SelectTier() and return
	// the appropriate tier based on bead priority and labels.
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	tests := []struct {
		name     string
		bead     *bead.Bead
		wantTier string
	}{
		{
			name:     "P0 gets high tier",
			bead:     &bead.Bead{ID: "p0-001", Priority: 0, Labels: []string{}},
			wantTier: provider.TierHigh,
		},
		{
			name:     "P1 gets medium tier",
			bead:     &bead.Bead{ID: "p1-001", Priority: 1, Labels: []string{}},
			wantTier: provider.TierMedium,
		},
		{
			name:     "P2 gets low tier",
			bead:     &bead.Bead{ID: "p2-001", Priority: 2, Labels: []string{}},
			wantTier: provider.TierLow,
		},
		{
			name:     "nil bead returns medium default",
			bead:     nil,
			wantTier: provider.TierMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectTier(cfg, tt.bead)
			if result != tt.wantTier {
				t.Errorf("SelectTier() = %q, want %q", result, tt.wantTier)
			}
		})
	}
}

// Expected failure: SelectTier function does not exist in escalation/ package yet
func TestSelectTier_NilConfigReturnsMedium(t *testing.T) {
	// SelectTier should return TierMedium as a safe default when config is nil.
	result := SelectTier(nil, &bead.Bead{ID: "test-001", Priority: 0})
	if result != provider.TierMedium {
		t.Errorf("SelectTier(nil config) = %q, want %q", result, provider.TierMedium)
	}
}

// Expected failure: SelectTier function does not exist in escalation/ package yet
func TestSelectTier_LabelOverride(t *testing.T) {
	// SelectTier should respect complexity label overrides that change the
	// tier regardless of priority.
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
			Labels: map[string]string{
				"complexity:high": provider.TierHigh,
				"complexity:low":  provider.TierLow,
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// P2 bead with complexity:high label should get high tier
	b := &bead.Bead{ID: "override-001", Priority: 2, Labels: []string{"complexity:high"}}
	result := SelectTier(cfg, b)
	if result != provider.TierHigh {
		t.Errorf("SelectTier(P2, complexity:high) = %q, want %q", result, provider.TierHigh)
	}
}

// Expected failure: SelectModel function does not exist in escalation/ package yet
func TestSelectModel_PriorityBasedSelection(t *testing.T) {
	// SelectModel should return the legacy model name based on bead priority.
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	tests := []struct {
		name      string
		bead      *bead.Bead
		wantModel string
	}{
		{
			name:      "P0 gets opus",
			bead:      &bead.Bead{ID: "p0", Priority: 0, Labels: []string{}},
			wantModel: "opus",
		},
		{
			name:      "P1 gets sonnet",
			bead:      &bead.Bead{ID: "p1", Priority: 1, Labels: []string{}},
			wantModel: "sonnet",
		},
		{
			name:      "P2 gets haiku",
			bead:      &bead.Bead{ID: "p2", Priority: 2, Labels: []string{}},
			wantModel: "haiku",
		},
		{
			name:      "nil bead gets sonnet default",
			bead:      nil,
			wantModel: "sonnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SelectModel(cfg, tt.bead)
			if result != tt.wantModel {
				t.Errorf("SelectModel() = %q, want %q", result, tt.wantModel)
			}
		})
	}
}

// Expected failure: SelectModel function does not exist in escalation/ package yet
func TestSelectModel_TestOnlyBeadRoutesToHaiku(t *testing.T) {
	// SelectModel should route test-only beads to haiku unless an explicit
	// complexity label overrides the selection.
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Test-only bead title patterns as defined in bead.IsTestOnlyBead()
	testOnlyTitles := []string{
		"Add tests for escalation handler",
		"Write tests for tier selection",
	}

	for _, title := range testOnlyTitles {
		b := &bead.Bead{ID: "test-only-001", Title: title, Priority: 0, Labels: []string{}}
		result := SelectModel(cfg, b)
		if result != "haiku" {
			t.Errorf("SelectModel(%q) = %q, want %q for test-only bead", title, result, "haiku")
		}
	}
}

// Expected failure: SelectModel function does not exist in escalation/ package yet
func TestSelectModel_TestOnlyBeadWithComplexityLabelOverrides(t *testing.T) {
	// When a test-only bead has an explicit complexity label, SelectModel
	// should respect the label override instead of defaulting to haiku.
	cfg := &config.Config{
		Models: config.ModelsConfig{
			Labels: map[string]string{
				"complexity:high": "opus",
			},
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	b := &bead.Bead{
		ID:       "test-override-001",
		Title:    "Add tests for complex feature",
		Priority: 1,
		Labels:   []string{"complexity:high"},
	}
	result := SelectModel(cfg, b)
	if result == "haiku" {
		t.Errorf("SelectModel() = %q, want override (not haiku) when complexity:high label present", result)
	}
}

// Expected failure: SelectModel function does not exist in escalation/ package yet
func TestSelectModel_NilConfigReturnsSonnet(t *testing.T) {
	// SelectModel should return "sonnet" as a safe default when config is nil.
	result := SelectModel(nil, &bead.Bead{ID: "test-001", Priority: 0})
	if result != "sonnet" {
		t.Errorf("SelectModel(nil config) = %q, want %q", result, "sonnet")
	}
}
