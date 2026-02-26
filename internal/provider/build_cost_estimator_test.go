package provider

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
)

func TestBuildProvidersFromConfig_WiresGeminiCostEstimator(t *testing.T) {
	t.Parallel()

	const (
		inputTokens = 100
		totalTokens = 150
	)

	providerDef := config.ProviderDef{
		Binary: "gemini",
		Models: map[string]string{
			TierLow: "gemini-2.5-flash",
		},
		CostPer1kInput:  0.5,
		CostPer1kOutput: 0.2,
	}

	cfg := &config.Config{
		Providers: map[string]config.ProviderDef{
			"gemini": providerDef,
		},
	}

	providers, err := BuildProvidersFromConfig(cfg)
	if err != nil {
		t.Fatalf("BuildProvidersFromConfig() error = %v", err)
	}

	gp, ok := providers["gemini"].(*GeminiProvider)
	if !ok {
		t.Fatalf("provider for %q is %T, want *GeminiProvider", "gemini", providers["gemini"])
	}

	jsonPayload := `{"response":"done","model":"gemini-2.5-flash","stats":{"models":{"gemini-2.5-flash":{"tokens":{"input":100,"prompt":0,"candidates":0,"total":150,"cached":0,"thoughts":0,"tool":0}}}}}`

	gp.runFn = func(ctx context.Context, binary string, args []string, prompt string, workDir string) (*geminiRunResult, error) {
		return &geminiRunResult{
			stdout:   []byte(jsonPayload),
			stderr:   []byte{},
			exitCode: 0,
			duration: 5 * time.Millisecond,
		}, nil
	}

	result, err := gp.Run(context.Background(), "test prompt", TierLow)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	expectedCost := providerDef.EstimateCostForModel("gemini-2.5-flash", inputTokens, totalTokens-inputTokens)
	if result.CostUSD != expectedCost {
		t.Fatalf("CostUSD = %v, want %v", result.CostUSD, expectedCost)
	}
}
