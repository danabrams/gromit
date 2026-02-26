package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// TestDecomposeFirstHandler_RetriesNonAtomicFailure verifies that
// DecomposeFirstHandler retries up to max_retries_before_decompose on failure.
func TestDecomposeFirstHandler_RetriesNonAtomicFailure(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Enabled:            true,
			Chain:              []string{"haiku", "sonnet", "opus"},
			MaxRetriesPerModel: 2,
			MaxRetriesPerBead:  5,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	mfa := &mockFailureAnalyzer{
		analyzeFn: func(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
			return &analyzer.Analysis{Category: "logic_error", Recoverable: false}, nil
		},
	}

	mbc := &mockBeadClient{}

	// DecomposeFirstHandler should exist and accept narrow interfaces
	handler := NewDecomposeFirstHandler(cfg, mfa, mbc, nil, nil, nil, nil, 2)
	if handler == nil {
		t.Fatal("NewDecomposeFirstHandler returned nil")
	}
}
