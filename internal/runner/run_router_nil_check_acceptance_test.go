//go:build acceptance

package runner

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// TestAcceptance_RunValidatesRouterNotNil verifies that Run() checks for nil router.
func TestAcceptance_RunValidatesRouterNotNil(t *testing.T) {
	cfg := &config.Config{Models: config.ModelsConfig{P1: "medium"}}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	r := &Runner{
		cfg:      cfg,
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		claude:   &mockClaudeClient{},
		router:   nil, // Explicitly nil router
	}

	err := r.Run(context.Background(), 1, time.Time{}, false)

	if err == nil {
		t.Fatal("Run() should return an error when router is nil")
	}
	if !strings.Contains(err.Error(), "router") {
		t.Errorf("Run() error should mention 'router', got: %q", err.Error())
	}
}

// TestAcceptance_RunChecksNilFieldsInOrder verifies all nil field checks.
func TestAcceptance_RunChecksNilFieldsInOrder(t *testing.T) {
	mockRouter := provider.NewSingleProviderRouter(&mockProviderForTest{})

	tests := []struct {
		r    *Runner
		want string
	}{
		{r: nil, want: "runner is nil"},
		{r: &Runner{cfg: nil, beads: &mockBeadClient{}, renderer: &mockRenderer{}, claude: &mockClaudeClient{}, router: mockRouter}, want: "config is nil"},
		{r: &Runner{cfg: &config.Config{}, beads: nil, renderer: &mockRenderer{}, claude: &mockClaudeClient{}, router: mockRouter}, want: "beads client is nil"},
		{r: &Runner{cfg: &config.Config{}, beads: &mockBeadClient{}, renderer: nil, claude: &mockClaudeClient{}, router: mockRouter}, want: "renderer is nil"},
		{r: &Runner{cfg: &config.Config{}, beads: &mockBeadClient{}, renderer: &mockRenderer{}, claude: nil, router: mockRouter}, want: "claude client is nil"},
		{r: &Runner{cfg: &config.Config{}, beads: &mockBeadClient{}, renderer: &mockRenderer{}, claude: &mockClaudeClient{}, router: nil}, want: "router"},
	}

	for _, tt := range tests {
		var err error
		if tt.r == nil {
			var r *Runner
			err = r.Run(context.Background(), 0, time.Time{}, false)
		} else {
			if tt.r.cfg != nil {
				tt.r.cfg.SetDefaults()
				tt.r.cfg.NormalizeNilFields()
			}
			err = tt.r.Run(context.Background(), 0, time.Time{}, false)
		}

		if err == nil {
			t.Fatalf("Run() should return error, got nil (want: %s)", tt.want)
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("Run() error should contain %q, got: %q", tt.want, err.Error())
		}
	}
}

// mockProviderForTest implements provider.Provider minimally for testing.
type mockProviderForTest struct{}

func (m *mockProviderForTest) Name() string { return "mock" }
func (m *mockProviderForTest) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}
func (m *mockProviderForTest) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer,
	handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}
func (m *mockProviderForTest) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true, Model: "mock-model"}, nil
}
func (m *mockProviderForTest) IsUsageLimitError(result *provider.Result, err error) bool {
	return false
}
