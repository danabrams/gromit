package prompt

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestShapeContextForBudget_UnderBudgetUnchanged(t *testing.T) {
	ctx := &Context{
		Bead:     &bead.Bead{ID: "test-1", Title: "Test bead"},
		ClaudeMD: "short",
		Rules:    "rule1",
	}
	ctx.normalizeNilFields()

	shaped, report := ShapeContextForBudget(ctx, 10000, 2000, "build")

	if shaped == nil {
		t.Fatal("expected non-nil shaped context")
	}
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.TrimActions) != 0 {
		t.Errorf("expected no trim actions, got %v", report.TrimActions)
	}
	if shaped.ClaudeMD != "short" {
		t.Errorf("expected ClaudeMD unchanged, got %q", shaped.ClaudeMD)
	}
	if shaped.Rules != "rule1" {
		t.Errorf("expected Rules unchanged, got %q", shaped.Rules)
	}
	if report.BeforeChars != report.AfterChars {
		t.Errorf("expected before == after chars, got %d != %d", report.BeforeChars, report.AfterChars)
	}
}
