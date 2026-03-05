//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/stage"
)

func TestValidateRetryRunsBuildBeforeRetry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callOrder := []string{}

	buildStage := &modelEscalatingStage{
		name:      "build",
		models:    []string{"haiku", "sonnet"},
		callOrder: &callOrder,
	}

	validateStage := &failingStage{
		name:      "validate",
		failCount: 1,
		callOrder: &callOrder,
	}

	beadLoop, err := loop.NewBeadLoop([]loop.StageSpec{
		{
			Stage: buildStage,
			Retry: stage.RetryConfig{MaxRetries: 2},
		},
		{
			Stage: validateStage,
			Retry: stage.RetryConfig{MaxRetries: 2, RetryWith: []string{"build"}},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	req := stage.Request{Bead: stage.BeadInfo{ID: "retry-test"}}
	if err := beadLoop.Run(ctx, req); err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	wantOrder := []string{"build", "validate", "build", "validate"}
	if !reflect.DeepEqual(callOrder, wantOrder) {
		t.Fatalf("call order = %v, want %v", callOrder, wantOrder)
	}

	wantModels := []string{"haiku", "sonnet"}
	if !reflect.DeepEqual(buildStage.ModelsUsed, wantModels) {
		t.Fatalf("build models = %v, want %v", buildStage.ModelsUsed, wantModels)
	}
}

func TestMaxRetriesExhaustionHaltsLoop(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	callOrder := []string{}

	buildStage := &modelEscalatingStage{
		name:      "build",
		models:    []string{"haiku"},
		callOrder: &callOrder,
	}

	validateStage := &failingStage{
		name:      "validate",
		failCount: 10,
		callOrder: &callOrder,
	}

	beadLoop, err := loop.NewBeadLoop([]loop.StageSpec{
		{
			Stage: buildStage,
			Retry: stage.RetryConfig{MaxRetries: 1},
		},
		{
			Stage: validateStage,
			Retry: stage.RetryConfig{MaxRetries: 0, RetryWith: []string{"build"}},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	if err := beadLoop.Run(ctx, stage.Request{Bead: stage.BeadInfo{ID: "max-retries"}}); err == nil || !errors.Is(err, loop.ErrMaxRetriesExceeded) {
		t.Fatalf("run bead loop = %v", err)
	}

	wantOrder := []string{"build", "validate"}
	if !reflect.DeepEqual(callOrder, wantOrder) {
		t.Fatalf("call order = %v, want %v", callOrder, wantOrder)
	}
}

func TestRetryContextCarriesPriorFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	buildStage := &modelEscalatingStage{
		name:   "build",
		models: []string{"haiku", "sonnet"},
	}
	validateStage := &failingStage{
		name:      "validate",
		failCount: 1,
	}

	beadLoop, err := loop.NewBeadLoop([]loop.StageSpec{
		{
			Stage: buildStage,
			Retry: stage.RetryConfig{MaxRetries: 2},
		},
		{
			Stage: validateStage,
			Retry: stage.RetryConfig{MaxRetries: 2, RetryWith: []string{"build"}},
		},
	})
	if err != nil {
		t.Fatalf("construct bead loop: %v", err)
	}

	if err := beadLoop.Run(ctx, stage.Request{Bead: stage.BeadInfo{ID: "retry-context"}}); err != nil {
		t.Fatalf("run bead loop: %v", err)
	}

	if len(validateStage.contexts) < 2 {
		t.Fatalf("validate contexts = %d, want >=2", len(validateStage.contexts))
	}

	if validateStage.contexts[0] != nil {
		t.Fatalf("expected first validate context to be nil, got %+v", validateStage.contexts[0])
	}

	secondCtx := validateStage.contexts[1]
	if secondCtx == nil {
		t.Fatal("expected retry validate context, got nil")
	}
	if secondCtx.Attempt != 1 {
		t.Fatalf("validate retry attempt = %d, want 1", secondCtx.Attempt)
	}
	if len(secondCtx.PriorFailures) == 0 || !strings.Contains(secondCtx.PriorFailures[0], "validate failed") {
		t.Fatalf("prior failures = %v", secondCtx.PriorFailures)
	}

	if len(buildStage.contexts) < 2 {
		t.Fatalf("build contexts = %d, want >=2", len(buildStage.contexts))
	}

	buildRetryCtx := buildStage.contexts[1]
	if buildRetryCtx == nil {
		t.Fatal("expected retry build context, got nil")
	}
	if buildRetryCtx.Attempt != 1 {
		t.Fatalf("build retry attempt = %d, want 1", buildRetryCtx.Attempt)
	}
	if len(buildRetryCtx.PriorFailures) == 0 || !strings.Contains(buildRetryCtx.PriorFailures[0], "validate failed") {
		t.Fatalf("build prior failures = %v", buildRetryCtx.PriorFailures)
	}
}

// Helpers used by multiple tests.
type modelEscalatingStage struct {
	name       string
	models     []string
	ModelsUsed []string
	callOrder  *[]string
	contexts   []*stage.RetryContext
}

func (m *modelEscalatingStage) Name() string {
	return m.name
}

func (m *modelEscalatingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	m.contexts = append(m.contexts, copyRetryContext(req.RetryContext))
	if m.callOrder != nil {
		*m.callOrder = append(*m.callOrder, m.name)
	}

	attempt := 0
	if req.RetryContext != nil {
		attempt = req.RetryContext.Attempt
	}

	idx := attempt
	if idx >= len(m.models) {
		idx = len(m.models) - 1
	}
	model := m.models[idx]
	m.ModelsUsed = append(m.ModelsUsed, model)

	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

type failingStage struct {
	name      string
	failCount int
	callOrder *[]string
	contexts  []*stage.RetryContext
	attempts  int
}

func (f *failingStage) Name() string {
	return f.name
}

func (f *failingStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	f.attempts++
	f.contexts = append(f.contexts, copyRetryContext(req.RetryContext))
	if f.callOrder != nil {
		*f.callOrder = append(*f.callOrder, f.name)
	}

	if f.attempts <= f.failCount {
		return nil, fmt.Errorf("%s failed attempt %d", f.name, f.attempts)
	}

	return &stage.Result{Decision: stage.DecisionProceed}, nil
}

func copyRetryContext(ctx *stage.RetryContext) *stage.RetryContext {
	if ctx == nil {
		return nil
	}
	cloned := &stage.RetryContext{
		Attempt:         ctx.Attempt,
		EscalationLevel: ctx.EscalationLevel,
		PriorFailures:   append([]string(nil), ctx.PriorFailures...),
	}
	return cloned
}
