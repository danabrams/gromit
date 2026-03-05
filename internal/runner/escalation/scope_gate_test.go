package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestShouldTriggerPreExecutionScopeDecomposition verifies that
// ShouldTriggerPreExecutionScopeDecomposition returns true when
// estimated file count > 3.
func TestShouldTriggerPreExecutionScopeDecomposition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		expectedOutputs []string
		wantDecompose   bool
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
			name:            "3 expected outputs (3 files) - at threshold",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go"},
			wantDecompose:   false,
		},
		{
			name:            "4 expected outputs (4 files) - exceeds threshold",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go", "util.go"},
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
			t.Parallel()
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

// TestCheckPostExecutionScope verifies post-execution scope checking
// against estimated file counts and hard caps.
func TestCheckPostExecutionScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		bc              *runtypes.BeadContext
		filesChanged    int
		wantExceeded    bool
		wantNonEmptyMsg bool
	}{
		{
			name: "WithinEstimate",
			bc: &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:              "test-id",
					Title:           "Test task",
					ExpectedOutputs: []string{"a.go", "b.go", "c.go"},
				},
			},
			filesChanged:    2,
			wantExceeded:    false,
			wantNonEmptyMsg: false,
		},
		{
			name: "ExceedsEstimate",
			bc: &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:              "test-id",
					Title:           "Test task",
					ExpectedOutputs: []string{"a.go", "b.go", "c.go"},
				},
			},
			filesChanged:    8,
			wantExceeded:    true,
			wantNonEmptyMsg: true,
		},
		{
			name: "NoEstimate_WithinCap",
			bc: &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:              "test-id",
					Title:           "Test task",
					ExpectedOutputs: []string{},
				},
			},
			filesChanged:    4,
			wantExceeded:    false,
			wantNonEmptyMsg: false,
		},
		{
			name: "NoEstimate_ExceedsCap",
			bc: &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:              "test-id",
					Title:           "Test task",
					ExpectedOutputs: []string{},
				},
			},
			filesChanged:    8,
			wantExceeded:    true,
			wantNonEmptyMsg: true,
		},
		{
			name:            "NilContext",
			bc:              nil,
			filesChanged:    10,
			wantExceeded:    false,
			wantNonEmptyMsg: false,
		},
		{
			name: "ExactBoundary",
			bc: &runtypes.BeadContext{
				Bead: &bead.Bead{
					ID:              "test-id",
					Title:           "Test task",
					ExpectedOutputs: []string{"a.go", "b.go", "c.go"},
				},
			},
			filesChanged:    6, // exactly 3 * 2 = 6, not exceeded
			wantExceeded:    false,
			wantNonEmptyMsg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := CheckPostExecutionScope(tt.bc, tt.filesChanged)
			if result.ScopeExceeded != tt.wantExceeded {
				t.Errorf("CheckPostExecutionScope().ScopeExceeded = %v, want %v", result.ScopeExceeded, tt.wantExceeded)
			}
			if tt.wantNonEmptyMsg && result.Message == "" {
				t.Error("CheckPostExecutionScope().Message is empty, want non-empty")
			}
			if !tt.wantNonEmptyMsg && result.Message != "" {
				t.Errorf("CheckPostExecutionScope().Message = %q, want empty", result.Message)
			}
		})
	}
}

// TestCheckPreExecutionScopeGate verifies that CheckPreExecutionScopeGate
// returns false and attempts decomposition when file count > 3.
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
			name:            "no decomposition needed (3 files, at threshold)",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go"},
			wantContinue:    true,
			wantDecomposed:  false,
		},
		{
			name:            "decomposition triggered (4 files)",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go", "util.go"},
			wantContinue:    false,
			wantDecomposed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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
