package prompt

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestSectionTaxonomyUniqueValues(t *testing.T) {
	sections := []string{
		SectionRules,
		SectionClaudeMD,
		SectionSpec,
		SectionConfirmedLearnings,
		SectionRecentLearnings,
		SectionTaskIdentity,
		SectionDiff,
		SectionFailureContext,
		SectionTemplateStatic,
		SectionSkillInstructions,
		SectionPlanBody,
		SectionRunStats,
		SectionBeadStats,
	}

	seen := map[string]bool{}
	for _, section := range sections {
		if section == "" {
			t.Fatal("section taxonomy values must be non-empty")
		}
		if seen[section] {
			t.Fatalf("duplicate section taxonomy value: %q", section)
		}
		seen[section] = true
	}
}

func TestNewDiagnostics(t *testing.T) {
	sectionTokens := map[string]int{
		SectionRules:              10,
		SectionSpec:               20,
		SectionConfirmedLearnings: 30,
	}

	diagnostics := NewDiagnostics("build", sectionTokens)

	if diagnostics.PromptType != "build" {
		t.Fatalf("PromptType = %q, want %q", diagnostics.PromptType, "build")
	}
	if diagnostics.EstimatedTokens != 60 {
		t.Fatalf("EstimatedTokens = %d, want %d", diagnostics.EstimatedTokens, 60)
	}
	if !reflect.DeepEqual(diagnostics.SectionTokens, sectionTokens) {
		t.Fatalf("SectionTokens = %#v, want %#v", diagnostics.SectionTokens, sectionTokens)
	}

	sectionTokens[SectionRules] = 999
	if diagnostics.SectionTokens[SectionRules] != 10 {
		t.Fatal("SectionTokens should be copied, not shared with input map")
	}
}

func TestNewDiagnosticsNilSectionTokens(t *testing.T) {
	diagnostics := NewDiagnostics("build", nil)

	if diagnostics.SectionTokens == nil {
		t.Fatal("SectionTokens must be non-nil")
	}
	if len(diagnostics.SectionTokens) != 0 {
		t.Fatalf("len(SectionTokens) = %d, want 0", len(diagnostics.SectionTokens))
	}
	if diagnostics.EstimatedTokens != 0 {
		t.Fatalf("EstimatedTokens = %d, want 0", diagnostics.EstimatedTokens)
	}
}

func TestPromptDiagnosticsJSONRoundTrip(t *testing.T) {
	original := &PromptDiagnostics{
		PromptType:      "build",
		EstimatedTokens: 120,
		SectionTokens: map[string]int{
			SectionRules: 40,
			SectionSpec:  80,
		},
		BudgetMaxChars:  30000,
		ShapeActions:    []string{"drop RecentLearnings", "truncate Spec"},
		PreShapeTokens:  150,
		PostShapeTokens: 120,
		ReportedTokens:  100,
		TokenDelta:      20,
		TokenDeltaPct:   20.0,
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded PromptDiagnostics
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(decoded, *original) {
		t.Fatalf("round-trip mismatch: got %#v want %#v", decoded, *original)
	}
}

func TestPromptDiagnosticsJSONOmitsOptionalFieldsWhenZero(t *testing.T) {
	diagnostics := NewDiagnostics("build", map[string]int{SectionRules: 5})

	encoded, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	omitted := []string{
		"budget_max_chars",
		"shape_actions",
		"pre_shape_tokens",
		"post_shape_tokens",
		"reported_tokens",
		"token_delta",
		"token_delta_pct",
	}
	for _, key := range omitted {
		if _, ok := raw[key]; ok {
			t.Fatalf("expected %q to be omitted for zero value JSON", key)
		}
	}

	if _, ok := raw["prompt_type"]; !ok {
		t.Fatal("expected prompt_type to be present")
	}
	if _, ok := raw["estimated_tokens"]; !ok {
		t.Fatal("expected estimated_tokens to be present")
	}
	if _, ok := raw["section_tokens"]; !ok {
		t.Fatal("expected section_tokens to be present")
	}
}

func TestPromptDiagnosticsReconcile(t *testing.T) {
	tests := []struct {
		name         string
		estimated    int
		reported     int
		wantDelta    int
		wantDeltaPct float64
	}{
		{
			name:         "reported non-zero positive delta",
			estimated:    120,
			reported:     100,
			wantDelta:    20,
			wantDeltaPct: 20,
		},
		{
			name:         "reported non-zero negative delta",
			estimated:    60,
			reported:     80,
			wantDelta:    -20,
			wantDeltaPct: -25,
		},
		{
			name:         "reported zero avoids division",
			estimated:    50,
			reported:     0,
			wantDelta:    50,
			wantDeltaPct: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := &PromptDiagnostics{EstimatedTokens: tt.estimated}
			diagnostics.Reconcile(tt.reported)

			if diagnostics.ReportedTokens != tt.reported {
				t.Fatalf("ReportedTokens = %d, want %d", diagnostics.ReportedTokens, tt.reported)
			}
			if diagnostics.TokenDelta != tt.wantDelta {
				t.Fatalf("TokenDelta = %d, want %d", diagnostics.TokenDelta, tt.wantDelta)
			}
			if math.Abs(diagnostics.TokenDeltaPct-tt.wantDeltaPct) > 1e-9 {
				t.Fatalf("TokenDeltaPct = %f, want %f", diagnostics.TokenDeltaPct, tt.wantDeltaPct)
			}
		})
	}
}

func TestPromptDiagnosticsReconcileNilReceiver(t *testing.T) {
	var diagnostics *PromptDiagnostics
	diagnostics.Reconcile(100)
}
