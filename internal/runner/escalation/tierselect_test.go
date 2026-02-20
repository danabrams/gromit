package escalation

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// --- Tier/model selection tests ---

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

func TestSelectTier_NilConfigReturnsMedium(t *testing.T) {
	// SelectTier should return TierMedium as a safe default when config is nil.
	result := SelectTier(nil, &bead.Bead{ID: "test-001", Priority: 0})
	if result != provider.TierMedium {
		t.Errorf("SelectTier(nil config) = %q, want %q", result, provider.TierMedium)
	}
}

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

func TestSelectTier_TestOnlyBeadRoutesToLow(t *testing.T) {
	// SelectTier should route test-only beads to low tier unless an explicit
	// complexity label overrides the selection.
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: provider.TierHigh,
			P1: provider.TierMedium,
			P2: provider.TierLow,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	testOnlyTitles := []string{
		"Add tests for escalation handler",
		"Write tests for tier selection",
	}

	for _, title := range testOnlyTitles {
		b := &bead.Bead{ID: "test-only-001", Title: title, Priority: 0, Labels: []string{}}
		result := SelectTier(cfg, b)
		if result != provider.TierLow {
			t.Errorf("SelectTier(%q) = %q, want %q for test-only bead", title, result, provider.TierLow)
		}
	}
}

func TestSelectTier_TestOnlyBeadWithComplexityLabelOverrides(t *testing.T) {
	// When a test-only bead has an explicit complexity label, SelectTier
	// should respect the label override instead of defaulting to low.
	cfg := &config.Config{
		Models: config.ModelsConfig{
			Labels: map[string]string{
				"complexity:high": provider.TierHigh,
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
	result := SelectTier(cfg, b)
	if result == provider.TierLow {
		t.Errorf("SelectTier() = %q, want override (not low) when complexity:high label present", result)
	}
}

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

func TestSelectModel_NilConfigReturnsSonnet(t *testing.T) {
	// SelectModel should return "sonnet" as a safe default when config is nil.
	result := SelectModel(nil, &bead.Bead{ID: "test-001", Priority: 0})
	if result != "sonnet" {
		t.Errorf("SelectModel(nil config) = %q, want %q", result, "sonnet")
	}
}

func TestCountLowComplexitySignals_EachSignal(t *testing.T) {
	one := 1

	tests := []struct {
		name string
		bead *bead.Bead
		want int
	}{
		{
			name: "title pattern",
			bead: &bead.Bead{
				Title:          "Rename config field to model tier",
				DependentCount: &one,
			},
			want: 1,
		},
		{
			name: "test-only title",
			bead: &bead.Bead{
				Title:          "Add tests for tier selection",
				DependentCount: &one,
			},
			want: 1,
		},
		{
			name: "tdd false label",
			bead: &bead.Bead{
				Title:          "Update escalation logic",
				Labels:         []string{lowComplexityTDDDisabledLabel},
				DependentCount: &one,
			},
			want: 1,
		},
		{
			name: "file count 1-3",
			bead: &bead.Bead{
				Title: "Update escalation logic",
				ExpectedOutputs: []string{
					"internal/runner/escalation/tierselect.go",
					"internal/runner/escalation/tierselect_test.go",
				},
				DependentCount: &one,
			},
			want: 1,
		},
		{
			name: "leaf bead",
			bead: &bead.Bead{
				Title: "Update escalation logic",
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLowComplexitySignals(&config.Config{}, tt.bead)
			if got != tt.want {
				t.Errorf("countLowComplexitySignals() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountLowComplexitySignals_NilBead(t *testing.T) {
	if got := countLowComplexitySignals(&config.Config{}, nil); got != 0 {
		t.Errorf("countLowComplexitySignals(nil) = %d, want 0", got)
	}
}

func TestIsLowComplexity_Threshold(t *testing.T) {
	one := 1

	tests := []struct {
		name string
		bead *bead.Bead
		want bool
	}{
		{
			name: "nil bead",
			bead: nil,
			want: false,
		},
		{
			name: "one signal is not enough",
			bead: &bead.Bead{
				Title:          "Rename helper",
				DependentCount: &one,
			},
			want: false,
		},
		{
			name: "two signals is low complexity",
			bead: &bead.Bead{
				Title:  "Update escalation logic",
				Labels: []string{lowComplexityTDDDisabledLabel},
			},
			want: true,
		},
		{
			name: "five signals are low complexity",
			bead: &bead.Bead{
				Title:  "Add tests for rename flow",
				Labels: []string{lowComplexityTDDDisabledLabel},
				ExpectedOutputs: []string{
					"internal/runner/escalation/tierselect.go",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLowComplexity(&config.Config{}, tt.bead)
			if got != tt.want {
				t.Errorf("isLowComplexity() = %t, want %t", got, tt.want)
			}
		})
	}
}
