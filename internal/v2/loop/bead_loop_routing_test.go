package loop

import (
	"context"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/v2/generation"
	"github.com/danabrams/gromit/internal/v2/llmtypes"
	"github.com/danabrams/gromit/internal/v2/routing"
	"github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/triage"
)

// spyStage captures Request fields from each invocation.
type spyStage struct {
	name     string
	models   []string
	hadProv  []bool
	decision stage.Decision
	failN    int // fail the first N calls
	calls    int
}

func (s *spyStage) Name() string { return s.name }

func (s *spyStage) RetryConfig() stage.RetryConfig {
	return stage.RetryConfig{MaxRetries: 3}
}

func (s *spyStage) Run(_ context.Context, req *stage.Request) (*stage.Result, error) {
	s.models = append(s.models, req.Model)
	s.hadProv = append(s.hadProv, req.Provider != nil)
	s.calls++
	if s.failN > 0 && s.calls <= s.failN {
		return &stage.Result{Decision: stage.DecisionFail}, nil
	}
	return &stage.Result{Decision: s.decision}, nil
}

// fakeProvider satisfies llmtypes.LLMProvider for testing.
type fakeProvider struct{}

func (f *fakeProvider) Invoke(_ context.Context, _ llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{}, nil
}
func (f *fakeProvider) StreamInvoke(_ context.Context, _ llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{}, nil
}

func TestBeadLoopRouterPopulatesProviderAndModel(t *testing.T) {
	t.Parallel()

	buildSpy := &spyStage{name: "build", decision: stage.DecisionProceed}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"claude": &fakeProvider{},
		},
		Ratio:    map[string]int{"claude": 1},
		Cooldown: time.Minute,
	})

	cfg := BeadLoopConfig{
		Gate:        newNoopStage("gate"),
		Build:       buildSpy,
		Validate:    newNoopStage("validate"),
		Review:      newNoopStage("review"),
		Epilogue:    newNoopStage("epilogue"),
		Router:      r,
		PhaseModels: map[string]string{"build": "low"},
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-1"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(buildSpy.models) == 0 {
		t.Fatal("build stage was never called")
	}
	if buildSpy.models[0] != "low" {
		t.Fatalf("build model = %q, want %q", buildSpy.models[0], "low")
	}
	if !buildSpy.hadProv[0] {
		t.Fatal("build Provider was nil, want non-nil")
	}
}

func TestBeadLoopEscalationBumpsTierOnRetry(t *testing.T) {
	t.Parallel()

	// Build fails once then succeeds — should see tier escalation.
	buildSpy := &spyStage{name: "build", decision: stage.DecisionProceed, failN: 1}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"claude": &fakeProvider{},
		},
		Ratio:    map[string]int{"claude": 1},
		Cooldown: time.Minute,
	})

	cfg := BeadLoopConfig{
		Gate:        newNoopStage("gate"),
		Build:       buildSpy,
		Validate:    newNoopStage("validate"),
		Review:      newNoopStage("review"),
		Epilogue:    newNoopStage("epilogue"),
		Router:      r,
		PhaseModels: map[string]string{"build": "low"},
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-1"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(buildSpy.models) < 2 {
		t.Fatalf("expected at least 2 build calls, got %d", len(buildSpy.models))
	}
	if buildSpy.models[0] != "low" {
		t.Fatalf("first attempt model = %q, want %q", buildSpy.models[0], "low")
	}
	if buildSpy.models[1] != "medium" {
		t.Fatalf("second attempt model = %q, want %q", buildSpy.models[1], "medium")
	}
}

func TestBeadLoopRoutingErrorFallsBackToDefault(t *testing.T) {
	t.Parallel()

	// Router with no providers — Select() returns ErrNoProviders.
	buildSpy := &spyStage{name: "build", decision: stage.DecisionProceed}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{}, // empty: triggers ErrNoProviders
		Cooldown:  time.Minute,
	})

	cfg := BeadLoopConfig{
		Gate:        newNoopStage("gate"),
		Build:       buildSpy,
		Validate:    newNoopStage("validate"),
		Review:      newNoopStage("review"),
		Epilogue:    newNoopStage("epilogue"),
		Router:      r,
		PhaseModels: map[string]string{"build": "low"},
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-fallback"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Stage must still execute (graceful degradation).
	if len(buildSpy.models) == 0 {
		t.Fatal("build stage was never called")
	}
	// Provider and Model should be empty since routing failed.
	if buildSpy.models[0] != "" {
		t.Fatalf("build model = %q, want empty (routing error fallback)", buildSpy.models[0])
	}
	if buildSpy.hadProv[0] {
		t.Fatal("build Provider should be nil when routing returns an error")
	}
}

func TestBeadLoopRoutingSkippedWhenRouterNil(t *testing.T) {
	t.Parallel()

	buildSpy := &spyStage{name: "build", decision: stage.DecisionProceed}

	cfg := BeadLoopConfig{
		Gate:     newNoopStage("gate"),
		Build:    buildSpy,
		Validate: newNoopStage("validate"),
		Review:   newNoopStage("review"),
		Epilogue: newNoopStage("epilogue"),
		// Router intentionally nil
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "bead-1"}}
	if _, err := loop.Run(context.Background(), beads, nil); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(buildSpy.models) == 0 {
		t.Fatal("build stage was never called")
	}
	// Without a router, the tier defaults to medium and resolves to "sonnet"
	// via the defaultTierToModel map. The model must always be set so the
	// stage's default provider uses the correct model for the configured tier.
	if buildSpy.models[0] != "sonnet" {
		t.Fatalf("build model = %q, want %q (default medium tier)", buildSpy.models[0], "sonnet")
	}
	if buildSpy.hadProv[0] {
		t.Fatal("build Provider should be nil when router is nil")
	}
}

func TestBeadLoopRoutingPopulatesProviderForTriage(t *testing.T) {
	t.Parallel()

	// Build always fails so triage is invoked.
	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:     "triage",
		category: triage.CategoryUnsafe, // terminates the loop with an error
	}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"claude": &fakeProvider{},
		},
		Ratio:    map[string]int{"claude": 1},
		Cooldown: time.Minute,
	})

	cfg := BeadLoopConfig{
		Gate:        newNoopStage("gate"),
		Build:       buildStage,
		Validate:    newNoopStage("validate"),
		Review:      newNoopStage("review"),
		Epilogue:    newNoopStage("epilogue"),
		Triage:      triageStage,
		Router:      r,
		PhaseModels: map[string]string{"triage": "medium"},
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "triage-routing", Labels: []string{generation.Format(0)}}}
	// Run will error because triage returns CategoryUnsafe, but triage still runs.
	_, _ = loop.Run(context.Background(), beads, nil)

	if triageStage.runCount == 0 {
		t.Fatal("triage stage was never called")
	}
	req := triageStage.requests[0]
	if req.Provider == nil {
		t.Fatal("triage Provider was nil, want non-nil from router")
	}
	if req.Model != "medium" {
		t.Fatalf("triage Model = %q, want %q", req.Model, "medium")
	}
}

func TestBarePhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"build", "build"},
		{"go:build", "build"},
		{"build:default", "build"},
		{"python:review", "review"},
		{"", ""},
		{"validate", "validate"},
		{"go:build:default", "go:build"},
	}
	for _, tt := range tests {
		if got := barePhase(tt.input); got != tt.want {
			t.Errorf("barePhase(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBeadLoopRoutingPopulatesProviderForDecompose(t *testing.T) {
	t.Parallel()

	// Build always fails so triage is invoked; triage returns decompose.
	buildStage := &decisionStage{name: "build", decision: stage.DecisionFail}
	triageStage := &fakeTriageStage{
		name:     "triage",
		category: triage.CategoryDecompose,
	}
	decomposeStage := &fakeBeadLoopDecomposeStage{
		name: "decompose",
		beads: []*bead.Bead{
			{ID: "sub-1", Labels: []string{generation.Format(1)}},
			{ID: "sub-2", Labels: []string{generation.Format(1)}},
		},
	}

	r := routing.NewRouter(routing.RouterConfig{
		Providers: map[string]llmtypes.LLMProvider{
			"claude": &fakeProvider{},
		},
		Ratio:    map[string]int{"claude": 1},
		Cooldown: time.Minute,
	})

	cfg := BeadLoopConfig{
		Gate:        newNoopStage("gate"),
		Build:       buildStage,
		Validate:    newNoopStage("validate"),
		Review:      newNoopStage("review"),
		Epilogue:    newNoopStage("epilogue"),
		Triage:      triageStage,
		Decompose:   decomposeStage,
		Router:      r,
		PhaseModels: map[string]string{"decompose": "high"},
	}

	loop, err := NewBeadLoop(cfg)
	if err != nil {
		t.Fatalf("NewBeadLoop: %v", err)
	}

	beads := []*bead.Bead{{ID: "decompose-routing", Labels: []string{generation.Format(0)}}}
	// Run proceeds through triage -> decompose; sub-beads then run through build (which fails again),
	// but we only care that decompose got routing applied.
	_, _ = loop.Run(context.Background(), beads, nil)

	if decomposeStage.runCount == 0 {
		t.Fatal("decompose stage was never called")
	}
	req := decomposeStage.requests[0]
	if req.Provider == nil {
		t.Fatal("decompose Provider was nil, want non-nil from router")
	}
	if req.Model != "high" {
		t.Fatalf("decompose Model = %q, want %q", req.Model, "high")
	}
}
