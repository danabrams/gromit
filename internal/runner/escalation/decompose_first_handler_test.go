package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestDecomposeFirstHandler_CreatesHandler verifies that
// DecomposeFirstHandler accepts narrow dependency interfaces.
func TestDecomposeFirstHandler_CreatesHandler(t *testing.T) {
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

// TestDecomposeFirstHandler_DecomposesAfterMaxRetries verifies that
// DecomposeFirstHandler triggers decomposition after exceeding max retries.
func TestDecomposeFirstHandler_DecomposesAfterMaxRetries(t *testing.T) {
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

	decomposeFn := func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
		return []runtypes.SubTask{}, nil
	}

	handler := NewDecomposeFirstHandler(cfg, mfa, mbc, decomposeFn, nil, nil, nil, 2)
	if handler == nil {
		t.Fatal("NewDecomposeFirstHandler returned nil")
	}

	// Handler should have the decomposeFn set
	if handler.decomposeFn == nil {
		t.Fatal("DecomposeFirstHandler.decomposeFn not set")
	}
}

// TestDecomposeFirstHandler_DecidesToDecomposeNonAtomic verifies that
// the handler decides to decompose when a non-atomic bead exceeds max retries.
func TestDecomposeFirstHandler_DecidesToDecomposeNonAtomic(t *testing.T) {
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

	handler := NewDecomposeFirstHandler(cfg, mfa, mbc, nil, nil, nil, nil, 2)
	if handler == nil {
		t.Fatal("NewDecomposeFirstHandler returned nil")
	}

	// ShouldDecomposeBeforeEscalate should exist and return a decision
	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-001", Title: "Test", Description: "Test bead"},
		RetriesThisModel:  3, // Exceeded max of 2
		MaxRetries:        2,
	}

	shouldDecompose := handler.ShouldDecomposeBeforeEscalate(bc)
	if !shouldDecompose {
		t.Fatal("expected ShouldDecomposeBeforeEscalate to return true when retries exceeded")
	}
}

// TestDecomposeFirstHandler_DetectsAtomicBead verifies that
// the handler can detect when a bead is atomic and should not be decomposed.
func TestDecomposeFirstHandler_DetectsAtomicBead(t *testing.T) {
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

	handler := NewDecomposeFirstHandler(cfg, mfa, mbc, nil, nil, nil, nil, 2)
	if handler == nil {
		t.Fatal("NewDecomposeFirstHandler returned nil")
	}

	// IsAtomicBead should exist
	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-001", Title: "Test", Description: "Test bead"},
		RetriesThisModel:  2,
		MaxRetries:        2,
	}

	isAtomic := handler.IsAtomicBead(bc)
	// When decomposeFn is nil, a bead is atomic (cannot be decomposed)
	if !isAtomic {
		t.Fatal("expected atomic bead when decomposeFn is nil")
	}
}

// TestDecomposeFirstHandler_DecomposesNonAtomicBead verifies that
// the handler decomposes non-atomic beads when max retries are exceeded.
func TestDecomposeFirstHandler_DecomposesNonAtomicBead(t *testing.T) {
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

	decomposeCalled := 0
	decomposeFn := func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
		decomposeCalled++
		return []runtypes.SubTask{}, nil
	}

	createSubCalled := 0
	createSubFn := func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
		createSubCalled++
		return nil
	}

	handler := NewDecomposeFirstHandler(cfg, mfa, mbc, decomposeFn, createSubFn, nil, nil, 2)
	if handler == nil {
		t.Fatal("NewDecomposeFirstHandler returned nil")
	}

	// Verify handler is not atomic (has decomposeFn)
	bc := &runtypes.BeadContext{
		Bead:              &bead.Bead{ID: "test-001", Title: "Test", Description: "Test bead"},
		RetriesThisModel:  2,
		MaxRetries:        2,
	}

	isAtomic := handler.IsAtomicBead(bc)
	if isAtomic {
		t.Fatal("expected non-atomic bead when decomposeFn is provided")
	}

	// Handler should be able to attempt decomposition
	continueLoop := handler.AttemptDecomposition(context.Background(), bc, "test failure")
	// After decomposition attempt, the loop should stop (return false)
	if continueLoop {
		t.Fatal("expected AttemptDecomposition to return false (stop loop)")
	}

	// Decomposition should have been called
	if decomposeCalled == 0 {
		t.Fatal("expected decomposeFn to be called")
	}

	// CreateSubFn should have been called
	if createSubCalled == 0 {
		t.Fatal("expected createSubFn to be called")
	}
}
