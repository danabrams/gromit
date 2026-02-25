package runner

import (
	"context"
	"io"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/provider"
)

func TestWorktreeMergerAdapterPendingBranches_NilManagerReturnsError(t *testing.T) {
	adapter := &worktreeMergerAdapter{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PendingBranches() panicked with nil manager: %v", r)
		}
	}()

	_, err := adapter.PendingBranches()
	if err == nil {
		t.Fatal("expected error for nil worktree manager")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "worktree manager") {
		t.Fatalf("error = %v, want message mentioning worktree manager", err)
	}
}

func TestApplyCostFallback_EstimatesFromProviderPricing(t *testing.T) {
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	defs := map[string]config.ProviderDef{
		"codex": {
			CostPer1kInput:  0.001,
			CostPer1kOutput: 0.002,
			ModelCosts: map[string]*config.ModelCost{
				"gpt-5.3-codex": {
					CostPer1kInput:  0.003,
					CostPer1kOutput: 0.012,
				},
			},
		},
	}

	applyCostFallback(result, "codex", defs)

	const want = 0.009
	if math.Abs(result.CostUSD-want) > 1e-12 {
		t.Fatalf("CostUSD = %v, want %v", result.CostUSD, want)
	}
}

func TestApplyCostFallback_DoesNotOverrideProviderReportedCost(t *testing.T) {
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0.25,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	defs := map[string]config.ProviderDef{
		"codex": {
			CostPer1kInput:  1,
			CostPer1kOutput: 1,
		},
	}

	applyCostFallback(result, "codex", defs)

	if result.CostUSD != 0.25 {
		t.Fatalf("CostUSD = %v, want unchanged 0.25", result.CostUSD)
	}
}

func TestApplyCostFallback_NoProviderDefLeavesZero(t *testing.T) {
	result := &provider.Result{
		Model:        "gpt-5.3-codex",
		CostUSD:      0,
		InputTokens:  1000,
		OutputTokens: 500,
	}

	applyCostFallback(result, "codex", map[string]config.ProviderDef{})

	if result.CostUSD != 0 {
		t.Fatalf("CostUSD = %v, want 0", result.CostUSD)
	}
}

type testTrendTrigger struct {
	triggered int
}

func (t *testTrendTrigger) Trigger() {
	t.triggered++
}

func TestIterationLogWriterAdapter_TriggersTrendRefreshOnSuccess(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "logs")
	l, err := logger.NewLogger(logDir)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	defer l.Close()

	trigger := &testTrendTrigger{}
	adapter := &iterationLogWriterAdapter{
		logger:       l,
		trendUpdater: trigger,
	}

	entry := &logger.IterationLog{
		Iteration: 1,
		BeadID:    "bead-1",
		Success:   true,
	}
	if err := adapter.Write(entry); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if trigger.triggered != 1 {
		t.Fatalf("Trigger count = %d, want 1", trigger.triggered)
	}
}

type trackingProvider struct {
	streamCalled bool
	runCalled    bool
}

func (p *trackingProvider) Name() string { return "tracking" }
func (p *trackingProvider) ModelForTier(tier string) string { return "sonnet" }
func (p *trackingProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	p.runCalled = true
	return &provider.Result{Success: true, Output: "ok"}, nil
}
func (p *trackingProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	p.streamCalled = true
	if output != nil {
		output.Write([]byte(prompt))
	}
	return &provider.Result{Success: true, Output: "ok"}, nil
}
func (p *trackingProvider) RunValidation(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
	return &provider.Result{Success: true}, nil
}
func (p *trackingProvider) IsUsageLimitError(result *provider.Result, err error) bool { return false }
func (p *trackingProvider) IsValidationPassed(result *provider.Result) bool { return result.Success }
func (p *trackingProvider) IsScopeTooLarge(result *provider.Result) (bool, string) { return false, "" }

func TestReviewInvokerAdapter_UsesStreamRun(t *testing.T) {
	prov := &trackingProvider{}
	router := provider.NewSingleProviderRouter(prov)
	adapter := &reviewInvokerAdapter{router: router}

	if _, err := adapter.StreamRun(context.Background(), "prompt", "high", io.Discard); err != nil {
		t.Fatalf("StreamRun returned error: %v", err)
	}

	if prov.runCalled {
		t.Fatal("expected Run() NOT to be called")
	}
	if !prov.streamCalled {
		t.Fatal("expected StreamRun() to be called")
	}
}
