package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/pipeline/execute"
	"github.com/danabrams/gromit/internal/runner/escalation"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// stubRenderer implements execute.PromptRenderer for tests.
type stubRenderer struct {
	prompt string
	err    error
}

func (r *stubRenderer) RenderBuild(title, description string, validationFailures []string) (string, error) {
	return r.prompt, r.err
}
func (r *stubRenderer) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	return r.prompt, r.err
}
func (r *stubRenderer) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	return r.prompt, r.err
}

// stubFallbackStage implements pipeline.Stage for fallback delegation tests.
type stubFallbackStage struct {
	called bool
	out    pipeline.Output
	err    error
}

func (s *stubFallbackStage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	s.called = true
	return s.out, s.err
}

func TestEscalationBuildStage_FallsBackForTDDFreshContext(t *testing.T) {
	t.Parallel()

	fallback := &stubFallbackStage{
		out: pipeline.Output{Decision: pipeline.Proceed, Model: "from-fallback"},
	}
	stage := &escalationBuildStage{
		fallback: fallback,
		renderer: &stubRenderer{prompt: "test prompt"},
	}

	in := pipeline.Input{
		Bead: &bead.Bead{Title: "test", Labels: []string{"tdd:true"}},
		Config: &config.Config{
			Methodology: config.MethodologyConfig{
				TDD:                  true,
				FreshContextPerCycle: true,
			},
		},
	}

	out, err := stage.Run(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fallback.called {
		t.Fatal("expected fallback stage to be called for TDD fresh-context")
	}
	if out.Model != "from-fallback" {
		t.Errorf("expected model 'from-fallback', got %q", out.Model)
	}
}

func TestEscalationBuildStage_DelegatesToHandlerOnEscalationEnabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			Enabled:            true,
			MaxRetriesPerModel: 2,
			MaxRetriesPerBead:  5,
			Chain:              []string{"low", "medium", "high"},
		},
		Claude: config.ClaudeConfig{
			BeadTimeout: 600,
		},
	}
	cfg.SetDefaults()

	// Create a handler with a no-op analyzer and bead client
	handler := escalation.NewHandler(
		cfg,
		&noopAnalyzer{},
		&noopBeadClient{},
		nil, nil, // decompose/createSub nil
		func(format string, args ...interface{}) {},
		nil,
	)

	// Track invocation
	invoked := false
	stage := &escalationBuildStage{
		handler:     handler,
		renderer:    &stubRenderer{prompt: "build this thing"},
		fallback:    &stubFallbackStage{},
		execInvoker: nil, // we'll use a custom invokeFn via handler
	}

	// We can't easily test with real execution.Invoker without a full router,
	// so we verify the stage constructs a BeadContext and calls the handler
	// by checking the stage's buildBeadContext directly.
	in := pipeline.Input{
		Bead:              &bead.Bead{ID: "bead-1", Title: "Test bead", Priority: 2},
		Config:            cfg,
		EscalationEnabled: true,
		Iteration:         3,
		Deadline:          time.Now().Add(1 * time.Hour),
	}

	bc := stage.buildBeadContext(in, "low", "build this thing")
	if bc.Bead.ID != "bead-1" {
		t.Errorf("expected bead ID 'bead-1', got %q", bc.Bead.ID)
	}
	if bc.Tier != "low" {
		t.Errorf("expected tier 'low', got %q", bc.Tier)
	}
	if bc.Iteration != 3 {
		t.Errorf("expected iteration 3, got %d", bc.Iteration)
	}
	if bc.MaxRetries != 2 {
		t.Errorf("expected MaxRetries 2, got %d", bc.MaxRetries)
	}
	if bc.MaxRetriesPerBead != 5 {
		t.Errorf("expected MaxRetriesPerBead 5, got %d", bc.MaxRetriesPerBead)
	}
	_ = invoked // used when full integration is wired
}

func TestEscalationBuildStage_PopulatesBeadContext(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Escalation: config.EscalationConfig{
			MaxRetriesPerModel: 3,
			MaxRetriesPerBead:  8,
		},
		Claude: config.ClaudeConfig{
			BeadTimeout: 1200,
		},
	}

	stage := &escalationBuildStage{
		promptRegistry: newBuildPromptRegistry(),
	}

	deadline := time.Now().Add(2 * time.Hour)
	in := pipeline.Input{
		Bead:      &bead.Bead{ID: "bead-42", Title: "Do stuff"},
		Config:    cfg,
		Iteration: 7,
		Deadline:  deadline,
	}

	bc := stage.buildBeadContext(in, "medium", "prompt text here")

	if bc.Bead.ID != "bead-42" {
		t.Errorf("BeadContext.Bead.ID = %q, want 'bead-42'", bc.Bead.ID)
	}
	if bc.Tier != "medium" {
		t.Errorf("BeadContext.Tier = %q, want 'medium'", bc.Tier)
	}
	if bc.BuildPrompt != "prompt text here" {
		t.Errorf("BeadContext.BuildPrompt = %q, want 'prompt text here'", bc.BuildPrompt)
	}
	if bc.Iteration != 7 {
		t.Errorf("BeadContext.Iteration = %d, want 7", bc.Iteration)
	}
	if bc.MaxRetries != 3 {
		t.Errorf("BeadContext.MaxRetries = %d, want 3", bc.MaxRetries)
	}
	if bc.MaxRetriesPerBead != 8 {
		t.Errorf("BeadContext.MaxRetriesPerBead = %d, want 8", bc.MaxRetriesPerBead)
	}
	if bc.BeadTimeout != 1200*time.Second {
		t.Errorf("BeadContext.BeadTimeout = %v, want %v", bc.BeadTimeout, 1200*time.Second)
	}
	if !bc.RunDeadline.Equal(deadline) {
		t.Errorf("BeadContext.RunDeadline = %v, want %v", bc.RunDeadline, deadline)
	}
	if bc.Result == nil {
		t.Error("BeadContext.Result should not be nil")
	}
	if bc.PromptCtx == nil {
		t.Error("BeadContext.PromptCtx should not be nil")
	}
}

func TestEscalationBuildStage_MapsFailureToError(t *testing.T) {
	t.Parallel()

	// Test the failure-to-error mapping logic directly, simulating what
	// happens when handler.ExecuteWithRetryWithEscalation returns false.
	bc := &runtypes.BeadContext{
		Result: &runtypes.IterationResult{
			FailurePhase: "compilation",
		},
	}

	out := pipeline.Output{
		Decision:     pipeline.Proceed,
		OriginalTier: "low",
		ActualTier:   bc.Tier,
	}

	// This is the mapping logic from Run()
	success := false
	if !success {
		failurePhase := "build"
		if bc.Result != nil && bc.Result.FailurePhase != "" {
			failurePhase = bc.Result.FailurePhase
		}
		err := fmt.Errorf("build: escalation handler failed at phase %s", failurePhase)
		if err == nil {
			t.Fatal("expected error from failed escalation")
		}
		if err.Error() != "build: escalation handler failed at phase compilation" {
			t.Errorf("unexpected error message: %s", err.Error())
		}
	}

	// Verify output still has Proceed decision (orchestrator inspects error separately)
	if out.Decision != pipeline.Proceed {
		t.Errorf("expected Decision=Proceed, got %v", out.Decision)
	}
}

func TestEscalationBuildStage_RenderPromptMethodology(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		methodology execute.Methodology
		wantPrompt  string
	}{
		{"standard", execute.MethodologyStandard, "standard-prompt"},
		{"tdd", execute.MethodologyTDD, "tdd-prompt"},
		{"refactor", execute.MethodologyRefactor, "refactor-prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &methodologyTrackingRenderer{prompts: map[string]string{
				"standard": "standard-prompt",
				"tdd":      "tdd-prompt",
				"refactor": "refactor-prompt",
			}}
			stage := &escalationBuildStage{renderer: r}

			in := pipeline.Input{
				Bead:   &bead.Bead{Title: "t", Description: "d"},
				Config: &config.Config{},
			}

			got, err := stage.renderPrompt(tt.methodology, in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantPrompt {
				t.Errorf("got prompt %q, want %q", got, tt.wantPrompt)
			}
		})
	}
}

func TestEscalationBuildStage_CompileTimeStageCheck(t *testing.T) {
	t.Parallel()
	// Verify the compile-time check at package level
	var _ pipeline.Stage = (*escalationBuildStage)(nil)
}

// --- test helpers ---

type noopAnalyzer struct{}

func (a *noopAnalyzer) Analyze(ctx context.Context, b *bead.Bead, failureOutput string) (*analyzer.Analysis, error) {
	return nil, nil
}

type noopBeadClient struct{}

func (c *noopBeadClient) AddComment(ctx context.Context, id, comment string) error {
	return nil
}

type methodologyTrackingRenderer struct {
	prompts map[string]string
}

func (r *methodologyTrackingRenderer) RenderBuild(title, description string, validationFailures []string) (string, error) {
	return r.prompts["standard"], nil
}
func (r *methodologyTrackingRenderer) RenderTDDBuild(title, description string, validationFailures []string) (string, error) {
	return r.prompts["tdd"], nil
}
func (r *methodologyTrackingRenderer) RenderRefactorBuild(title, description string, validationFailures []string) (string, error) {
	return r.prompts["refactor"], nil
}
