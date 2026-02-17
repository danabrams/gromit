package prompt

import (
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

func makeLearning(content string) learnings.Learning {
	return learnings.Learning{Content: content, Category: "patterns"}
}

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

func TestShapeContextForBudget_DropRecentLearningsFirst(t *testing.T) {
	// Create a context where RecentLearnings push it over budget
	ctx := &Context{
		Bead:            &bead.Bead{ID: "b1", Title: "T"},
		ClaudeMD:        "claude md content",
		Rules:           "rules content",
		RecentLearnings: []learnings.Learning{makeLearning("recent learning that is large enough")},
	}
	ctx.normalizeNilFields()

	// Set budget just under total so RecentLearnings must be dropped
	total := measureContext(ctx)
	budget := total - 10

	shaped, report := ShapeContextForBudget(ctx, budget, 2000, "build")

	if len(shaped.RecentLearnings) != 0 {
		t.Errorf("expected RecentLearnings dropped, got %d items", len(shaped.RecentLearnings))
	}
	if shaped.ClaudeMD != "claude md content" {
		t.Errorf("expected ClaudeMD preserved, got %q", shaped.ClaudeMD)
	}
	if len(report.TrimActions) == 0 {
		t.Fatal("expected at least one trim action")
	}
	if report.TrimActions[0] != "drop RecentLearnings" {
		t.Errorf("expected first trim action to be 'drop RecentLearnings', got %q", report.TrimActions[0])
	}
	if report.AfterChars >= report.BeforeChars {
		t.Errorf("expected AfterChars < BeforeChars, got %d >= %d", report.AfterChars, report.BeforeChars)
	}
}

func TestShapeContextForBudget_DropClaudeMDSecond(t *testing.T) {
	// ClaudeMD is large, no RecentLearnings — should still drop ClaudeMD as step 2
	ctx := &Context{
		Bead:     &bead.Bead{ID: "b1", Title: "T"},
		ClaudeMD: strings.Repeat("x", 500),
		Rules:    "rules",
	}
	ctx.normalizeNilFields()

	// Budget is less than total but more than total-without-ClaudeMD
	total := measureContext(ctx)
	budget := total - 100

	shaped, report := ShapeContextForBudget(ctx, budget, 2000, "build")

	if shaped.ClaudeMD != "" {
		t.Errorf("expected ClaudeMD dropped, got %d chars", len(shaped.ClaudeMD))
	}
	found := false
	for _, a := range report.TrimActions {
		if a == "drop ClaudeMD" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'drop ClaudeMD' in trim actions, got %v", report.TrimActions)
	}
}

func TestShapeContextForBudget_CapConfirmedLearningsThird(t *testing.T) {
	// 3 learnings of 100 chars each, cap to 150 chars should keep only some
	ctx := &Context{
		Bead:     &bead.Bead{ID: "b1", Title: "T"},
		Rules:    "rules",
		ClaudeMD: strings.Repeat("c", 200),
		ConfirmedLearnings: []learnings.Learning{
			makeLearning(strings.Repeat("a", 100)),
			makeLearning(strings.Repeat("b", 100)),
			makeLearning(strings.Repeat("c", 100)),
		},
	}
	ctx.normalizeNilFields()

	// Budget forces steps 1+2, and then step 3 (cap learnings to 150)
	// After steps 1+2: bead(3) + rules(5) + learnings(300) = 308
	// Budget that requires capping learnings but not dropping them entirely
	budget := 200

	shaped, report := ShapeContextForBudget(ctx, budget, 150, "build")

	// ConfirmedLearnings should be capped, not empty
	if len(shaped.ConfirmedLearnings) == 0 {
		t.Fatal("expected ConfirmedLearnings capped, not fully dropped")
	}
	if len(shaped.ConfirmedLearnings) >= 3 {
		t.Errorf("expected fewer than 3 ConfirmedLearnings, got %d", len(shaped.ConfirmedLearnings))
	}
	// Total learnings chars should be <= learningCapChars
	totalLearningChars := 0
	for _, l := range shaped.ConfirmedLearnings {
		totalLearningChars += len(l.Content)
	}
	if totalLearningChars > 150 {
		t.Errorf("expected learnings capped to 150 chars, got %d", totalLearningChars)
	}
	found := false
	for _, a := range report.TrimActions {
		if a == "cap ConfirmedLearnings" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'cap ConfirmedLearnings' in trim actions, got %v", report.TrimActions)
	}
}
