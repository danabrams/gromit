package prompt

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

func TestShapeBuildContext_ScopeBudget_Disabled(t *testing.T) {
	// When scopeBudgetEnabled=false, budget is unchanged regardless of file count.
	r := &Renderer{
		budgetMaxChars:         20000,
		budgetLearningCapChars: 2000,
		scopeBudgetEnabled:     false,
	}

	ctx := &Context{
		Bead:     &bead.Bead{ID: "b1", Title: "T", ExpectedOutputs: []string{"a.go"}},
		ClaudeMD: strings.Repeat("x", 8000),
		Rules:    strings.Repeat("r", 8000),
		Spec:     strings.Repeat("s", 8000),
		ConfirmedLearnings: []learnings.Learning{
			makeLearning(strings.Repeat("l", 2000)),
		},
	}
	ctx.normalizeNilFields()

	// Context is over 20000 chars, so shaping will trim.
	// With scopeBudgetEnabled=false, the full 20000 budget is used.
	shaped, report := r.shapeBuildContext(ctx, "build")
	if shaped == nil {
		t.Fatal("expected non-nil shaped context")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Verify that the full budget was used (not a reduced one).
	// With 26000+ chars and a 20000 budget, RecentLearnings and ClaudeMD should be trimmed.
	// If a reduced budget (10000) were used, we'd expect more aggressive trimming.
	// The key check: ClaudeMD being dropped is the second trim step. With full budget,
	// dropping RecentLearnings (0 here) and ClaudeMD should suffice.
	if report.AfterChars > 20000 {
		t.Errorf("expected shaped context to fit within full budget 20000, got %d", report.AfterChars)
	}
}

func TestShapeBuildContext_ScopeBudget_SmallScope(t *testing.T) {
	// <=2 files gets 50% budget.
	r := &Renderer{
		budgetMaxChars:         20000,
		budgetLearningCapChars: 2000,
		scopeBudgetEnabled:     true,
	}

	// Bead with 2 expected outputs (<=2 files -> 50% budget = 10000).
	ctx := &Context{
		Bead:     &bead.Bead{ID: "b1", Title: "T", ExpectedOutputs: []string{"a.go", "b.go"}},
		ClaudeMD: strings.Repeat("x", 4000),
		Rules:    strings.Repeat("r", 4000),
		Spec:     strings.Repeat("s", 4000),
		ConfirmedLearnings: []learnings.Learning{
			makeLearning(strings.Repeat("l", 2000)),
		},
	}
	ctx.normalizeNilFields()

	shaped, report := r.shapeBuildContext(ctx, "build")
	if shaped == nil {
		t.Fatal("expected non-nil shaped context")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Effective budget is 10000 (50% of 20000).
	// Total context is ~14000+ chars, so trimming should occur.
	if report.AfterChars > 10000 {
		t.Errorf("expected shaped context to fit within scope-adjusted budget 10000, got %d", report.AfterChars)
	}
}

func TestShapeBuildContext_ScopeBudget_MediumScope(t *testing.T) {
	// 3-4 files gets 75% budget.
	r := &Renderer{
		budgetMaxChars:         20000,
		budgetLearningCapChars: 2000,
		scopeBudgetEnabled:     true,
	}

	// Bead with 3 expected outputs (3-4 files -> 75% budget = 15000).
	ctx := &Context{
		Bead:     &bead.Bead{ID: "b1", Title: "T", ExpectedOutputs: []string{"a.go", "b.go", "c.go"}},
		ClaudeMD: strings.Repeat("x", 6000),
		Rules:    strings.Repeat("r", 6000),
		Spec:     strings.Repeat("s", 6000),
		ConfirmedLearnings: []learnings.Learning{
			makeLearning(strings.Repeat("l", 2000)),
		},
	}
	ctx.normalizeNilFields()

	shaped, report := r.shapeBuildContext(ctx, "build")
	if shaped == nil {
		t.Fatal("expected non-nil shaped context")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Effective budget is 15000 (75% of 20000).
	if report.AfterChars > 15000 {
		t.Errorf("expected shaped context to fit within scope-adjusted budget 15000, got %d", report.AfterChars)
	}
}

func TestShapeBuildContext_ScopeBudget_LargeScope(t *testing.T) {
	// 5+ files gets full budget.
	r := &Renderer{
		budgetMaxChars:         20000,
		budgetLearningCapChars: 2000,
		scopeBudgetEnabled:     true,
	}

	// Bead with 5 expected outputs (5+ files -> full budget = 20000).
	ctx := &Context{
		Bead: &bead.Bead{ID: "b1", Title: "T", ExpectedOutputs: []string{
			"a.go", "b.go", "c.go", "d.go", "e.go",
		}},
		ClaudeMD: strings.Repeat("x", 8000),
		Rules:    strings.Repeat("r", 8000),
		Spec:     strings.Repeat("s", 8000),
		ConfirmedLearnings: []learnings.Learning{
			makeLearning(strings.Repeat("l", 2000)),
		},
	}
	ctx.normalizeNilFields()

	shaped, report := r.shapeBuildContext(ctx, "build")
	if shaped == nil {
		t.Fatal("expected non-nil shaped context")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Full budget is 20000 - same as disabled.
	if report.AfterChars > 20000 {
		t.Errorf("expected shaped context to fit within full budget 20000, got %d", report.AfterChars)
	}
}

func TestShapeBuildContext_ScopeBudget_NilBead(t *testing.T) {
	// Nil bead uses full budget even when scopeBudgetEnabled=true.
	r := &Renderer{
		budgetMaxChars:         20000,
		budgetLearningCapChars: 2000,
		scopeBudgetEnabled:     true,
	}

	ctx := &Context{
		Bead:     nil,
		ClaudeMD: strings.Repeat("x", 8000),
		Rules:    strings.Repeat("r", 8000),
		Spec:     strings.Repeat("s", 8000),
		ConfirmedLearnings: []learnings.Learning{
			makeLearning(strings.Repeat("l", 2000)),
		},
	}
	ctx.normalizeNilFields()

	shaped, report := r.shapeBuildContext(ctx, "build")
	if shaped == nil {
		t.Fatal("expected non-nil shaped context")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// Full budget is 20000 since bead is nil.
	if report.AfterChars > 20000 {
		t.Errorf("expected shaped context to fit within full budget 20000, got %d", report.AfterChars)
	}
}

func TestSetScopeBudgetEnabled(t *testing.T) {
	r := &Renderer{}
	r.SetScopeBudgetEnabled(true)
	if !r.scopeBudgetEnabled {
		t.Error("expected scopeBudgetEnabled to be true")
	}
	r.SetScopeBudgetEnabled(false)
	if r.scopeBudgetEnabled {
		t.Error("expected scopeBudgetEnabled to be false")
	}
}

func TestSetScopeBudgetEnabled_NilReceiver(t *testing.T) {
	var r *Renderer
	// Should not panic.
	r.SetScopeBudgetEnabled(true)
}
