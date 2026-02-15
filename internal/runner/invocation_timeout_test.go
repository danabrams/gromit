package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestExecuteClaudeInvocation_UsesConfigInvocationTimeout(t *testing.T) {
	var observed time.Duration
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				return nil, fmt.Errorf("missing invocation deadline")
			}
			observed = time.Until(deadline)
			return &provider.Result{Success: true, Model: "test-sonnet", Output: "ok"}, nil
		},
	}
	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	cfg := &config.Config{
		Claude: config.ClaudeConfig{
			Timeout:            2,
			StallTimeout:       1,
			StallTimeoutActive: 1,
			BeadTimeout:        30,
		},
		Validation: config.ValidationConfig{Enabled: false},
		Review:     config.ReviewConfig{Enabled: false},
	}

	var buf bytes.Buffer
	r, err := NewRunnerWithDeps(cfg, &buf, t.TempDir(), Deps{
		Beads:    &mockBeadClient{},
		Router:   mockRouter,
		Analyzer: &mockFailureAnalyzer{},
		Renderer: &mockPromptRenderer{},
		Logger:   &mockIterationLogger{},
	})
	if err != nil {
		t.Fatalf("NewRunnerWithDeps failed: %v", err)
	}

	bc := &runtypes.BeadContext{
		Bead:        &bead.Bead{ID: "bead-1", Title: "Test"},
		Tier:        provider.TierMedium,
		Result:      &IterationResult{},
		BuildPrompt: "prompt",
		ParentCtx:   context.Background(),
	}

	_, _, _, _, err = r.executeClaudeInvocation(context.Background(), bc)
	if err != nil {
		t.Fatalf("executeClaudeInvocation failed: %v", err)
	}
	if observed == 0 {
		t.Fatal("expected invocation deadline to be set")
	}
	if observed < time.Second || observed > 4*time.Second {
		t.Fatalf("invocation deadline = %v, want ~2s", observed)
	}
}
