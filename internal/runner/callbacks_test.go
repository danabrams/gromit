package runner

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

func TestSummarizeATDDProviderOutput(t *testing.T) {
	if got := summarizeATDDProviderOutput("   "); got != "no provider output" {
		t.Fatalf("expected empty output sentinel, got %q", got)
	}

	if got := summarizeATDDProviderOutput("  failure details  "); got != "failure details" {
		t.Fatalf("expected trimmed output, got %q", got)
	}

	long := strings.Repeat("x", 1700)
	got := summarizeATDDProviderOutput(long)
	if !strings.Contains(got, "...[truncated]...") {
		t.Fatalf("expected truncated marker, got %q", got)
	}
	if len(got) >= len(long) {
		t.Fatalf("expected summarized output shorter than input, got len=%d input=%d", len(got), len(long))
	}
}

func TestMakeMethodologyExec_ATDDCapturesStreamCostData(t *testing.T) {
	mockProvider := &mockProviderWithRouterTracking{
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			if handler != nil {
				handler([]byte(`{"type":"result","subtype":"done","total_cost_usd":1.23,"input_tokens":10,"output_tokens":20}`))
			}
			return &provider.Result{Success: true}, nil
		},
	}

	streamLogger, err := logger.NewStreamLogger(t.TempDir())
	if err != nil {
		t.Fatalf("NewStreamLogger: %v", err)
	}
	defer func() {
		_ = streamLogger.Close()
	}()

	r := &Runner{
		router:       provider.NewSingleProviderRouter(mockProvider),
		output:       io.Discard,
		streamLogger: streamLogger,
		renderer: &mockPromptRenderer{
			RenderAcceptanceTestsFn: func(ctx *prompt.Context) (string, error) {
				return "acceptance prompt", nil
			},
		},
	}
	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "b1"},
		Result:    &runtypes.IterationResult{},
		Tier:      provider.TierLow,
		PromptCtx: &prompt.Context{},
	}

	methExec := r.makeMethodologyExec()
	if methExec == nil {
		t.Fatal("expected makeMethodologyExec to return executor")
	}

	if err := methExec.RunAcceptanceTests(context.Background(), bc); err != nil {
		t.Fatalf("RunAcceptanceTests returned error: %v", err)
	}

	if bc.Result.InputTokens != 10 {
		t.Fatalf("InputTokens = %d, want 10", bc.Result.InputTokens)
	}
	if bc.Result.OutputTokens != 20 {
		t.Fatalf("OutputTokens = %d, want 20", bc.Result.OutputTokens)
	}
	if bc.Result.CostUSD != 1.23 {
		t.Fatalf("CostUSD = %.2f, want 1.23", bc.Result.CostUSD)
	}
}
