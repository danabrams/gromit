package escalation

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// TestExecuteWithRetry_TriggersPreExecutionScopeGate verifies that
// ExecuteWithRetry calls CheckPreExecutionScopeGate before attempting
// the first invocation, and decomposes if file count > 2.
func TestExecuteWithRetry_TriggersPreExecutionScopeGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		expectedOutputs []string
		wantDecomposed  bool
	}{
		{
			name:            "no scope gate needed (1 file)",
			expectedOutputs: []string{"main.go"},
			wantDecomposed:  false,
		},
		{
			name:            "scope gate triggered (3 files)",
			expectedOutputs: []string{"main.go", "main_test.go", "helper.go"},
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

			cfg := &config.Config{
				Escalation: config.EscalationConfig{
					Enabled: true,
				},
			}

			bc := &runtypes.BeadContext{
				Bead: b,
				Tier: "medium",
				Result: &runtypes.IterationResult{
					BeadID: b.ID,
				},
			}

			// Track invocation
			invokeCalled := false
			invokeFn := func(ctx context.Context, bc *runtypes.BeadContext, prompt string) (*runtypes.InvocationResult, error) {
				invokeCalled = true
				return &runtypes.InvocationResult{
					ProviderResult: &provider.Result{
						Success: true,
						Output:  "Success",
					},
				}, nil
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

			handler := NewHandler(cfg, nil, nil, decomposeFn, createSubFn, nil, nil)
			ctx := context.Background()

			success := handler.ExecuteWithRetry(ctx, bc, invokeFn)

			// Check if decomposition was triggered as expected
			if tt.wantDecomposed {
				if !decomposeCalled {
					t.Errorf("Expected decompose to be called, but it wasn't")
				}
				if !bc.Result.Decomposed {
					t.Errorf("Expected Result.Decomposed=true, got false")
				}
				// Invoke should NOT be called when decomposition is triggered
				if invokeCalled {
					t.Errorf("Expected invoke NOT to be called when decomposing, but it was")
				}
				// ExecuteWithRetry returns false after decomposition
				if success {
					t.Errorf("Expected ExecuteWithRetry to return false after decomposition, got true")
				}
			} else {
				if decomposeCalled {
					t.Errorf("Expected decompose NOT to be called, but it was")
				}
				// Invoke should be called when no decomposition is needed
				if !invokeCalled {
					t.Errorf("Expected invoke to be called, but it wasn't")
				}
				if success != true {
					t.Errorf("Expected ExecuteWithRetry to return true for successful invocation")
				}
			}
		})
	}
}
