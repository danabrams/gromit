package runner

import (
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestRunner_AcceptsLabelFilters tests that Runner can accept optional spec labels
func TestRunner_AcceptsLabelFilters(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mock := &mockBeadClient{
		ReadyFn: func() (*bead.Bead, error) {
			return nil, nil // No work
		},
	}

	deps := Deps{
		Beads:    mock,
		Claude:   &mockClaudeClient{},
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	}

	r, err := NewRunnerWithDeps(cfg, io.Discard, "/tmp/gromit", deps)
	if err != nil {
		t.Fatalf("NewRunnerWithDeps() error = %v", err)
	}

	// Test that Runner can be configured with label filters
	labels := []string{"spec:auth", "spec:payments"}
	r.SetLabelFilters(labels)

	// Verify labels were stored
	if len(r.labelFilters) != 2 {
		t.Errorf("Expected 2 label filters, got %d", len(r.labelFilters))
	}
	if r.labelFilters[0] != "spec:auth" {
		t.Errorf("Expected first label 'spec:auth', got %s", r.labelFilters[0])
	}
	if r.labelFilters[1] != "spec:payments" {
		t.Errorf("Expected second label 'spec:payments', got %s", r.labelFilters[1])
	}
}
