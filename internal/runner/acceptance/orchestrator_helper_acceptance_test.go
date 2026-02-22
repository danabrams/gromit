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

// TestNewOrchestratorTestHelper_ReturnsNonNil verifies that NewOrchestratorTestHelper
// returns a non-nil helper when given a valid config.
func TestNewOrchestratorTestHelper_ReturnsNonNil(t *testing.T) {
	cfg := &config.Config{}
	h := NewOrchestratorTestHelper(t, cfg, io.Discard)
	if h == nil {
		t.Fatal("expected non-nil OrchestratorTestHelper")
	}
}

// TestNewOrchestratorTestHelperWithDeps_ReturnsNonNil verifies that
// NewOrchestratorTestHelperWithDeps returns a non-nil helper when given
// mock BeadClient and Router.
func TestNewOrchestratorTestHelperWithDeps_ReturnsNonNil(t *testing.T) {
	cfg := &config.Config{}
	beads := &mockBeadClient{}
	router := newMockRouter()
	h := NewOrchestratorTestHelperWithDeps(t, cfg, io.Discard, beads, router)
	if h == nil {
		t.Fatal("expected non-nil OrchestratorTestHelper")
	}
}

// TestOrchestratorTestHelper_Run_DelegatesToOrchestrator verifies that Run
// delegates to Orchestrator.Run by confirming GetBead is called via beads.Ready.
func TestOrchestratorTestHelper_Run_DelegatesToOrchestrator(t *testing.T) {
	getCalled := false
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			getCalled = true
			return nil, nil
		},
	}
	h := NewOrchestratorTestHelperWithDeps(t, &config.Config{}, io.Discard, beads, newMockRouter())
	err := h.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !getCalled {
		t.Error("expected Orchestrator to call GetBead via beads.Ready")
	}
}

// TestOrchestratorTestHelper_SetLabelFilters_CallsReadyWithLabel verifies that
// after SetLabelFilters is called, Run uses ReadyWithLabel instead of Ready.
func TestOrchestratorTestHelper_SetLabelFilters_CallsReadyWithLabel(t *testing.T) {
	var calledLabel string
	beads := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			t.Error("Ready() should not be called when label filters are set")
			return nil, nil
		},
		ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
			calledLabel = label
			return nil, nil
		},
	}
	h := NewOrchestratorTestHelperWithDeps(t, &config.Config{}, io.Discard, beads, newMockRouter())
	h.SetLabelFilters([]string{"spec:auth"})
	err := h.Run(context.Background(), 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if calledLabel != "spec:auth" {
		t.Errorf("expected ReadyWithLabel called with 'spec:auth', got %q", calledLabel)
	}
}
