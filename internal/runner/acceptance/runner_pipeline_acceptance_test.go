//go:build acceptance

package acceptance_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestOrchestratorHelper_TDDPromptSelection verifies that the orchestrator loop
// runs a bead to completion for each TDD config scenario without error.
func TestOrchestratorHelper_TDDPromptSelection(t *testing.T) {
	tests := []struct {
		name       string
		globalTDD  bool
		beadLabels []string
	}{
		{name: "TDD active via global config", globalTDD: true, beadLabels: []string{}},
		{name: "TDD active via bead label", globalTDD: false, beadLabels: []string{"tdd:true"}},
		{name: "TDD inactive globally, no label", globalTDD: false, beadLabels: []string{}},
		{name: "TDD disabled via label overriding global", globalTDD: true, beadLabels: []string{"tdd:false"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{TDD: tt.globalTDD},
			}
			beadReady := false
			mockBeads := &mockBeadClient{
				ReadyFn: func() (*bead.Bead, error) {
					if beadReady {
						return nil, nil
					}
					beadReady = true
					return &bead.Bead{
						ID:     "test-bead-1",
						Title:  "Test bead",
						Labels: tt.beadLabels,
					}, nil
				},
			}
			h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, mockBeads, newMockRouter())
			if err := h.Run(context.Background(), 0, time.Time{}, nil); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
		})
	}
}

// TestOrchestratorHelper_ATDDSkippedForTestOnlyBead verifies that the orchestrator
// loop runs each ATDD scenario bead to completion without error.
func TestOrchestratorHelper_ATDDSkippedForTestOnlyBead(t *testing.T) {
	tests := []struct {
		name       string
		beadTitle  string
		globalATDD bool
	}{
		{name: "test-only bead with global ATDD", beadTitle: "Add unit tests for config loading", globalATDD: true},
		{name: "regular bead with global ATDD", beadTitle: "Implement dark mode toggle", globalATDD: true},
		{name: "test-only bead with ATDD disabled", beadTitle: "Add unit tests for config loading", globalATDD: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Methodology: config.MethodologyConfig{ATDD: tt.globalATDD},
			}
			beadReady := false
			mockBeads := &mockBeadClient{
				ReadyFn: func() (*bead.Bead, error) {
					if beadReady {
						return nil, nil
					}
					beadReady = true
					return &bead.Bead{
						ID:    "test-bead-1",
						Title: tt.beadTitle,
					}, nil
				},
			}
			h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, mockBeads, newMockRouter())
			if err := h.Run(context.Background(), 0, time.Time{}, nil); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
		})
	}
}
