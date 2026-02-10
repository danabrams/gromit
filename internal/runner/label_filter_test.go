package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

// labelFilterTestOpts configures a label filtering test case.
type labelFilterTestOpts struct {
	labelFilters       []string
	readyWithLabelFunc func(label string) (*bead.Bead, error)
	readyFunc          func() (*bead.Bead, error)
}

// labelFilterTestState tracks calls made during a label filtering test.
type labelFilterTestState struct {
	ReadyWithLabelCalls []string
	ReadyCalled         bool
}

// setupLabelFilterTest creates a Runner configured for label filter testing.
// It returns the runner and a state object tracking mock call history.
func setupLabelFilterTest(t *testing.T, opts labelFilterTestOpts) (*Runner, *labelFilterTestState) {
	t.Helper()

	state := &labelFilterTestState{}

	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			state.ReadyWithLabelCalls = append(state.ReadyWithLabelCalls, label)
			if opts.readyWithLabelFunc != nil {
				return opts.readyWithLabelFunc(label)
			}
			return nil, nil
		},
		ReadyFunc: func() (*bead.Bead, error) {
			state.ReadyCalled = true
			if opts.readyFunc != nil {
				return opts.readyFunc()
			}
			return nil, nil
		},
	}

	cfg := &config.Config{}
	cfg.SetDefaults()

	r := &Runner{
		cfg:          cfg,
		beads:        mock,
		labelFilters: opts.labelFilters,
	}

	return r, state
}

// TestRunner_SetLabelFilters verifies that SetLabelFilters stores label filters correctly
func TestRunner_SetLabelFilters(t *testing.T) {
	r := &Runner{}

	labels := []string{"spec:auth", "spec:payments"}
	r.SetLabelFilters(labels)

	if len(r.labelFilters) != 2 {
		t.Errorf("Expected 2 label filters, got %d", len(r.labelFilters))
	}
	if r.labelFilters[0] != "spec:auth" {
		t.Errorf("Expected first label to be 'spec:auth', got %s", r.labelFilters[0])
	}
	if r.labelFilters[1] != "spec:payments" {
		t.Errorf("Expected second label to be 'spec:payments', got %s", r.labelFilters[1])
	}
}

// TestRunner_GetNextBead_LabelFiltering is a table-driven test covering all
// label filtering behaviors in getNextBead().
func TestRunner_GetNextBead_LabelFiltering(t *testing.T) {
	authBead := &bead.Bead{
		ID: "auth-001", Title: "Auth task", Priority: 1,
		Labels: []string{"spec:auth"}, ExpectedOutputs: []string{},
	}
	payBead := &bead.Bead{
		ID: "pay-001", Title: "Payment task", Priority: 0,
		Labels: []string{"spec:payments"}, ExpectedOutputs: []string{},
	}
	multiLabelBead := &bead.Bead{
		ID: "multi-001", Title: "Multi-label task", Priority: 1,
		Labels: []string{"spec:auth", "spec:payments"}, ExpectedOutputs: []string{},
	}

	tests := []struct {
		name               string
		labelFilters       []string
		readyWithLabelFunc func(string) (*bead.Bead, error)
		readyFunc          func() (*bead.Bead, error)
		wantBeadID         string // empty means nil result expected
		wantReadyCalled    bool
		wantLabelCalls     []string // expected ReadyWithLabel call labels
	}{
		{
			name:         "no filters uses Ready",
			labelFilters: []string{},
			readyFunc: func() (*bead.Bead, error) {
				return authBead, nil
			},
			wantBeadID:      "auth-001",
			wantReadyCalled: true,
			wantLabelCalls:  nil,
		},
		{
			name:         "empty filter list uses Ready",
			labelFilters: []string{},
			readyFunc: func() (*bead.Bead, error) {
				return authBead, nil
			},
			wantBeadID:      "auth-001",
			wantReadyCalled: true,
			wantLabelCalls:  nil,
		},
		{
			name:         "single label filter uses ReadyWithLabel",
			labelFilters: []string{"spec:auth"},
			readyWithLabelFunc: func(label string) (*bead.Bead, error) {
				if label == "spec:auth" {
					return authBead, nil
				}
				return nil, nil
			},
			wantBeadID:     "auth-001",
			wantLabelCalls: []string{"spec:auth"},
		},
		{
			name:         "multiple labels first has bead",
			labelFilters: []string{"spec:auth", "spec:payments"},
			readyWithLabelFunc: func(label string) (*bead.Bead, error) {
				if label == "spec:auth" {
					return authBead, nil
				}
				return nil, nil
			},
			wantBeadID:     "auth-001",
			wantLabelCalls: []string{"spec:auth", "spec:payments"},
		},
		{
			name:         "multiple labels second has bead",
			labelFilters: []string{"spec:auth", "spec:payments"},
			readyWithLabelFunc: func(label string) (*bead.Bead, error) {
				if label == "spec:payments" {
					return payBead, nil
				}
				return nil, nil
			},
			wantBeadID:     "pay-001",
			wantLabelCalls: []string{"spec:auth", "spec:payments"},
		},
		{
			name:         "multiple labels none have beads",
			labelFilters: []string{"spec:auth", "spec:payments", "spec:reporting"},
			readyWithLabelFunc: func(label string) (*bead.Bead, error) {
				return nil, nil
			},
			wantBeadID:     "",
			wantLabelCalls: []string{"spec:auth", "spec:payments", "spec:reporting"},
		},
		{
			name:         "multiple labels picks highest priority",
			labelFilters: []string{"spec:auth", "spec:payments"},
			readyWithLabelFunc: func(label string) (*bead.Bead, error) {
				if label == "spec:auth" {
					return authBead, nil // P1
				}
				if label == "spec:payments" {
					return payBead, nil // P0
				}
				return nil, nil
			},
			wantBeadID:     "pay-001",
			wantLabelCalls: []string{"spec:auth", "spec:payments"},
		},
		{
			name:         "deduplicates same bead from multiple labels",
			labelFilters: []string{"spec:auth", "spec:payments"},
			readyWithLabelFunc: func(label string) (*bead.Bead, error) {
				return multiLabelBead, nil
			},
			wantBeadID:     "multi-001",
			wantLabelCalls: []string{"spec:auth", "spec:payments"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, state := setupLabelFilterTest(t, labelFilterTestOpts{
				labelFilters:       tt.labelFilters,
				readyWithLabelFunc: tt.readyWithLabelFunc,
				readyFunc:          tt.readyFunc,
			})

			result, err := r.getNextBead()
			if err != nil {
				t.Fatalf("getNextBead() error: %v", err)
			}

			// Check expected bead
			if tt.wantBeadID == "" {
				if result != nil {
					t.Errorf("expected nil bead, got %q", result.ID)
				}
			} else {
				if result == nil {
					t.Fatalf("expected bead %q, got nil", tt.wantBeadID)
				}
				if result.ID != tt.wantBeadID {
					t.Errorf("expected bead %q, got %q", tt.wantBeadID, result.ID)
				}
			}

			// Check Ready() was called (or not)
			if tt.wantReadyCalled && !state.ReadyCalled {
				t.Error("expected Ready() to be called")
			}
			if !tt.wantReadyCalled && state.ReadyCalled {
				t.Error("expected Ready() NOT to be called")
			}

			// Check ReadyWithLabel call order
			if tt.wantLabelCalls != nil {
				if len(state.ReadyWithLabelCalls) != len(tt.wantLabelCalls) {
					t.Errorf("expected %d ReadyWithLabel calls, got %d: %v",
						len(tt.wantLabelCalls), len(state.ReadyWithLabelCalls), state.ReadyWithLabelCalls)
				}
				for i, want := range tt.wantLabelCalls {
					if i < len(state.ReadyWithLabelCalls) && state.ReadyWithLabelCalls[i] != want {
						t.Errorf("ReadyWithLabel call %d: got %q, want %q", i, state.ReadyWithLabelCalls[i], want)
					}
				}
			}
		})
	}
}
