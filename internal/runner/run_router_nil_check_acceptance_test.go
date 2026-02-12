//go:build acceptance

package runner

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_RunValidatesRouterNotNil verifies that Run() checks for nil router
// and returns an error before attempting to process beads.
// Expected failure: Run() does not yet validate r.router field
func TestAcceptance_RunValidatesRouterNotNil(t *testing.T) {
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0: "high",
			P1: "medium",
			P2: "low",
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create a runner with nil router (simulating incomplete initialization)
	r := &Runner{
		cfg:      cfg,
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		claude:   &mockClaudeClient{},
		router:   nil, // Explicitly nil router
		output:   os.Stdout,
	}

	// Expected failure: Run() does not yet validate r.router field
	err := r.Run(context.Background(), 1, time.Time{}, false)

	if err == nil {
		t.Fatal("Run() should return an error when router is nil, got nil error")
	}

	if !strings.Contains(err.Error(), "router") {
		t.Errorf("Run() error should mention 'router', got: %q", err.Error())
	}
}

// TestAcceptance_RunChecksNilFieldsBeforeRouterCheck verifies that Run() validates
// other critical fields (cfg, beads, renderer, claude) before router.
// Expected failure: Run() does not yet validate r.router field (but already validates others)
func TestAcceptance_RunChecksNilFieldsBeforeRouterCheck(t *testing.T) {
	tests := []struct {
		name        string
		runner      *Runner
		wantErrText string
	}{
		{
			name:        "nil runner returns error",
			runner:      nil,
			wantErrText: "runner is nil",
		},
		{
			name: "nil config returns error",
			runner: &Runner{
				cfg:      nil,
				beads:    &mockBeadClient{},
				renderer: &mockRenderer{},
				claude:   &mockClaudeClient{},
				router:   mockRouterForTest(),
				output:   os.Stdout,
			},
			wantErrText: "config is nil",
		},
		{
			name: "nil beads returns error",
			runner: &Runner{
				cfg: &config.Config{
					Models: config.ModelsConfig{P1: "medium"},
				},
				beads:    nil,
				renderer: &mockRenderer{},
				claude:   &mockClaudeClient{},
				router:   mockRouterForTest(),
				output:   os.Stdout,
			},
			wantErrText: "beads client is nil",
		},
		{
			name: "nil renderer returns error",
			runner: &Runner{
				cfg: &config.Config{
					Models: config.ModelsConfig{P1: "medium"},
				},
				beads:    &mockBeadClient{},
				renderer: nil,
				claude:   &mockClaudeClient{},
				router:   mockRouterForTest(),
				output:   os.Stdout,
			},
			wantErrText: "renderer is nil",
		},
		{
			name: "nil claude returns error",
			runner: &Runner{
				cfg: &config.Config{
					Models: config.ModelsConfig{P1: "medium"},
				},
				beads:    &mockBeadClient{},
				renderer: &mockRenderer{},
				claude:   nil,
				router:   mockRouterForTest(),
				output:   os.Stdout,
			},
			wantErrText: "claude client is nil",
		},
		{
			name: "nil router returns error",
			runner: &Runner{
				cfg: &config.Config{
					Models: config.ModelsConfig{P1: "medium"},
				},
				beads:    &mockBeadClient{},
				renderer: &mockRenderer{},
				claude:   &mockClaudeClient{},
				router:   nil, // Expected failure: Run() does not yet validate r.router field
				output:   os.Stdout,
			},
			wantErrText: "router",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.runner != nil && tt.runner.cfg != nil {
				tt.runner.cfg.SetDefaults()
				tt.runner.cfg.NormalizeNilFields()
			}

			var err error
			if tt.runner == nil {
				var r *Runner
				err = r.Run(context.Background(), 0, time.Time{}, false)
			} else {
				err = tt.runner.Run(context.Background(), 0, time.Time{}, false)
			}

			if err == nil {
				t.Fatalf("Run() should return an error for %s, got nil", tt.name)
			}

			if !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("Run() error should contain %q, got: %q", tt.wantErrText, err.Error())
			}
		})
	}
}

// mockRouterForTest creates a minimal router for testing nil router validation.
// We use NewSingleProviderRouter with a mock provider to create a valid router instance.
func mockRouterForTest() *provider.Router {
	// Create a mock provider
	mockProv := &mockProviderForRouterTest{}
	return provider.NewSingleProviderRouter(mockProv)
}

// mockProviderForRouterTest implements the provider.Provider interface minimally
type mockProviderForRouterTest struct{}

func (m *mockProviderForRouterTest) Name() string { return "mock" }

func (m *mockProviderForRouterTest) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRouterTest) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRouterTest) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}

func (m *mockProviderForRouterTest) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
