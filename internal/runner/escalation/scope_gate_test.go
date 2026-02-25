package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestShouldTriggerPreExecutionScopeDecomposition verifies that
// ShouldTriggerPreExecutionScopeDecomposition returns true when
// estimated file count > 2.
func TestShouldTriggerPreExecutionScopeDecomposition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		expectedOutputs  []string
		wantDecompose    bool
	}{
		{
			name:            "no expected outputs (0 files)",
			expectedOutputs: []string{},
			wantDecompose:   false,
		},
		{
			name:            "1 expected output (1 file)",
			expectedOutputs: []string{"main.go"},
			wantDecompose:   false,
		},
		{
			name:            "2 expected outputs (2 files)",
			expectedOutputs: []string{"main.go", "main_test.go"},
			wantDecompose:   false,
		},
		{
			name:            "3 expected outputs (3 files) - exceeds threshold",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go"},
			wantDecompose:   true,
		},
		{
			name:            "5 expected outputs (5 files)",
			expectedOutputs: []string{"a.go", "b.go", "c.go", "d.go", "e.go"},
			wantDecompose:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{
				ID:              "test-id",
				Title:           "Test task",
				Description:     "Test description",
				ExpectedOutputs: tt.expectedOutputs,
			}

			bc := &runtypes.BeadContext{
				Bead: b,
			}

			got := ShouldTriggerPreExecutionScopeDecomposition(bc)
			if got != tt.wantDecompose {
				t.Errorf("ShouldTriggerPreExecutionScopeDecomposition() = %v, want %v", got, tt.wantDecompose)
			}
		})
	}
}

// TestCheckPreExecutionScopeGate verifies that CheckPreExecutionScopeGate
// returns false and attempts decomposition when file count > 2.
func TestCheckPreExecutionScopeGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		expectedOutputs []string
		wantContinue    bool
		wantDecomposed  bool
	}{
		{
			name:            "no decomposition needed (1 file)",
			expectedOutputs: []string{"main.go"},
			wantContinue:    true,
			wantDecomposed:  false,
		},
		{
			name:            "decomposition triggered (3 files)",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go"},
			wantContinue:    false,
			wantDecomposed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &bead.Bead{
				ID:              "test-id",
				Title:           "Test task",
				Description:     "Test description",
				ExpectedOutputs: tt.expectedOutputs,
			}

			bc := &runtypes.BeadContext{
				Bead: b,
				Result: &runtypes.IterationResult{
					BeadID: b.ID,
				},
			}

			// Mock decompose function
			decomposeCalled := false
			decomposeFn := func(ctx context.Context, b *bead.Bead) ([]runtypes.SubTask, error) {
				decomposeCalled = true
				return []runtypes.SubTask{}, nil
			}

			// Mock create sub function
			createSubFn := func(ctx context.Context, b *bead.Bead, tasks []runtypes.SubTask) error {
				return nil
			}

			handler := NewHandler(
				nil,
				nil,
				nil,
				decomposeFn,
				createSubFn,
				nil,
				nil,
			)

			ctx := context.Background()
			got := handler.CheckPreExecutionScopeGate(ctx, bc)

			if got != tt.wantContinue {
				t.Errorf("CheckPreExecutionScopeGate() = %v, want %v", got, tt.wantContinue)
			}

			if tt.wantDecomposed && !decomposeCalled {
				t.Errorf("Expected decompose to be called, but it wasn't")
			}

			if tt.wantDecomposed != (bc.Result != nil && bc.Result.Decomposed) {
				t.Errorf("Result.Decomposed = %v, want %v", bc.Result.Decomposed, tt.wantDecomposed)
			}
		})
	}
}
