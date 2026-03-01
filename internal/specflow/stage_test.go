package specflow

import (
	"errors"
	"testing"
)

func TestValidateTransition(t *testing.T) {
	legalTransitions := []struct {
		name string
		from Stage
		to   Stage
	}{
		{"planning->acceptance-tests", StagePlanning, StageAcceptanceTests},
		{"acceptance-tests->implementation", StageAcceptanceTests, StageImplementation},
		{"implementation->local-gate", StageImplementation, StageLocalGate},
		{"local-gate->review", StageLocalGate, StageReview},
		{"review->global-gate", StageReview, StageGlobalGate},
		{"global-gate->done", StageGlobalGate, StageDone},
		{"drafting->review", StageDrafting, StageReview},
	}

	for _, tt := range legalTransitions {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateTransition(tt.from, tt.to); err != nil {
				t.Fatalf("ValidateTransition(%s, %s) = %v, want nil", tt.from, tt.to, err)
			}
		})
	}

	illegalTransitions := []struct {
		name string
		from Stage
		to   Stage
	}{
		{"planning->implementation", StagePlanning, StageImplementation},
		{"review->done", StageReview, StageDone},
		{"done->planning", StageDone, StagePlanning},
		{"unknown->planning", Stage("unknown"), StagePlanning},
		{"drafting->planning", StageDrafting, StagePlanning},
	}

	for _, tt := range illegalTransitions {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransition(tt.from, tt.to)
			if err == nil {
				t.Fatalf("expected transition %s -> %s to fail", tt.from, tt.to)
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("expected ErrInvalidTransition for %s -> %s, got %v", tt.from, tt.to, err)
			}
		})
	}
}
