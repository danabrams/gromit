package runner

import (
	"context"
	"io"
	"testing"

	"github.com/danabrams/gromit/internal/provider"
)

func TestRunRefactorWithRouter_UsesStreamRun(t *testing.T) {
	runCalled := false
	streamCalled := false
	gotNilHandler := false

	p := &mockProviderWithRouterTracking{
		name: "mock-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			runCalled = true
			return &provider.Result{Success: true, Model: "run-model"}, nil
		},
		streamRunFn: func(ctx context.Context, prompt, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
			streamCalled = true
			gotNilHandler = handler == nil
			if handler != nil {
				handler([]byte(`{"type":"system","subtype":"thread.started"}`))
			}
			return &provider.Result{Success: true, Model: "stream-model"}, nil
		},
	}

	r := &Runner{
		router: provider.NewSingleProviderRouter(p),
		output: io.Discard,
	}

	result, err := r.runRefactorWithRouter(context.Background(), "refactor prompt", provider.TierMedium)
	if err != nil {
		t.Fatalf("runRefactorWithRouter() error = %v", err)
	}
	if result == nil {
		t.Fatal("runRefactorWithRouter() returned nil result")
	}
	if runCalled {
		t.Fatal("runRefactorWithRouter() called Run(); want StreamRun only")
	}
	if !streamCalled {
		t.Fatal("runRefactorWithRouter() did not call StreamRun()")
	}
	if gotNilHandler {
		t.Fatal("runRefactorWithRouter() passed nil stream handler; want non-nil for streaming visibility")
	}
}
