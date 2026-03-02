package methodology

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

// TestAdapterSelectionMatrix verifies that the correct adapter is selected
// for each supported profile.
func TestAdapterSelectionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile         string
		expectedAdapter string
	}{
		{"go", "GoAdapter"},
		{"Go", "GoAdapter"},
		{"GO", "GoAdapter"},
		{"node", "PassthroughAdapter"},
		{"Node", "PassthroughAdapter"},
		{"NODE", "PassthroughAdapter"},
		{"python", "PassthroughAdapter"},
		{"Python", "PassthroughAdapter"},
		{"PYTHON", "PassthroughAdapter"},
		{"custom", "PassthroughAdapter"},
		{"Custom", "PassthroughAdapter"},
		{"CUSTOM", "PassthroughAdapter"},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			adapter, err := ResolveAdapter(tt.profile)
			if err != nil {
				t.Fatalf("ResolveAdapter(%q) returned error: %v", tt.profile, err)
			}

			switch tt.expectedAdapter {
			case "GoAdapter":
				if _, ok := adapter.(GoAdapter); !ok {
					t.Errorf("ResolveAdapter(%q) returned type %T, want GoAdapter", tt.profile, adapter)
				}
			case "PassthroughAdapter":
				if _, ok := adapter.(PassthroughAdapter); !ok {
					t.Errorf("ResolveAdapter(%q) returned type %T, want PassthroughAdapter", tt.profile, adapter)
				}
			}
		})
	}
}

// TestAdapterCommandTransformationMatrix verifies that each adapter transforms
// commands appropriately for each phase.
func TestAdapterCommandTransformationMatrix(t *testing.T) {
	t.Parallel()

	testCommands := []string{"go test ./...", "go vet ./..."}
	testPackages := []string{"internal/runner"}

	tests := []struct {
		name             string
		adapter          RunnerAdapter
		phase            string
		expectedModified bool
	}{
		{
			name:             "GoAdapter.Acceptance modifies commands",
			adapter:          GoAdapter{},
			phase:            "Acceptance",
			expectedModified: true,
		},
		{
			name:             "GoAdapter.TDD preserves commands",
			adapter:          GoAdapter{},
			phase:            "TDD",
			expectedModified: false,
		},
		{
			name:             "GoAdapter.Validation preserves commands",
			adapter:          GoAdapter{},
			phase:            "Validation",
			expectedModified: false,
		},
		{
			name:             "PassthroughAdapter.Acceptance preserves commands",
			adapter:          PassthroughAdapter{},
			phase:            "Acceptance",
			expectedModified: false,
		},
		{
			name:             "PassthroughAdapter.TDD preserves commands",
			adapter:          PassthroughAdapter{},
			phase:            "TDD",
			expectedModified: false,
		},
		{
			name:             "PassthroughAdapter.Validation preserves commands",
			adapter:          PassthroughAdapter{},
			phase:            "Validation",
			expectedModified: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result []string
			switch tt.phase {
			case "Acceptance":
				result = tt.adapter.Acceptance(testCommands, testPackages)
			case "TDD":
				result = tt.adapter.TDD(testCommands, testPackages)
			case "Validation":
				result = tt.adapter.Validation(testCommands, testPackages)
			}

			modified := result[0] != testCommands[0]
			if modified != tt.expectedModified {
				t.Errorf("adapter phase %s modified=%v, want modified=%v", tt.phase, modified, tt.expectedModified)
			}
		})
	}
}

// TestExecutorAdapterSelectionFromConfig verifies that NewExecutor selects
// the correct adapter based on config profile.
func TestExecutorAdapterSelectionFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile         string
		expectedAdapter string
	}{
		{"go", "GoAdapter"},
		{"node", "PassthroughAdapter"},
		{"python", "PassthroughAdapter"},
		{"custom", "PassthroughAdapter"},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			cfg := &config.Config{
				Project: config.ProjectConfig{
					Profile: tt.profile,
				},
			}

			executor := NewExecutor(cfg, nil, nil, nil, nil)
			if executor == nil {
				t.Fatal("NewExecutor returned nil")
			}

			switch tt.expectedAdapter {
			case "GoAdapter":
				if _, ok := executor.adapter.(GoAdapter); !ok {
					t.Errorf("NewExecutor with profile %q selected adapter type %T, want GoAdapter", tt.profile, executor.adapter)
				}
			case "PassthroughAdapter":
				if _, ok := executor.adapter.(PassthroughAdapter); !ok {
					t.Errorf("NewExecutor with profile %q selected adapter type %T, want PassthroughAdapter", tt.profile, executor.adapter)
				}
			}
		})
	}
}

// TestExecutorAdapterProfileTracking verifies that the executor correctly
// tracks which profile was selected.
func TestExecutorAdapterProfileTracking(t *testing.T) {
	t.Parallel()

	tests := []struct {
		profile         string
		expectedProfile string
	}{
		{"go", "go"},
		{"node", "node"},
		{"python", "python"},
		{"custom", "custom"},
		{"Go", "Go"},
		{"NODE", "NODE"},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			cfg := &config.Config{
				Project: config.ProjectConfig{
					Profile: tt.profile,
				},
			}

			executor := NewExecutor(cfg, nil, nil, nil, nil)
			if executor.adapterProfile != tt.expectedProfile {
				t.Errorf("NewExecutor adapterProfile=%q, want %q", executor.adapterProfile, tt.expectedProfile)
			}
		})
	}
}
