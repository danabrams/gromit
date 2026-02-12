package prompt

import (
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
)

func TestContextNormalizeNilFieldsInitializesRecentValidationFailures(t *testing.T) {
	// Expected failure: RecentValidationFailures field does not exist on Context yet

	ctx := &Context{
		Iteration: 1,
		Model:     "sonnet",
	}

	// Before normalization, the field should be nil (zero value)
	if ctx.RecentValidationFailures != nil {
		t.Error("expected RecentValidationFailures to start as nil")
	}

	ctx.normalizeNilFields()

	// After normalization, RecentValidationFailures should be an empty slice, not nil
	if ctx.RecentValidationFailures == nil {
		t.Error("expected RecentValidationFailures to be non-nil after normalization")
	}
	if len(ctx.RecentValidationFailures) != 0 {
		t.Errorf("expected empty RecentValidationFailures, got %d items", len(ctx.RecentValidationFailures))
	}
}

func TestContextNormalizeNilFieldsPreservesExistingValidationFailures(t *testing.T) {
	// Expected failure: RecentValidationFailures field does not exist on Context yet

	ctx := &Context{
		ConfirmedLearnings:       []learnings.Learning{{Content: "a"}},
		RecentLearnings:          []learnings.Learning{{Content: "b"}},
		RecentValidationFailures: []string{"--- FAIL: TestFoo", "FAIL\tpkg/foo"},
	}

	ctx.normalizeNilFields()

	// Existing validation failures should be preserved
	if len(ctx.RecentValidationFailures) != 2 {
		t.Errorf("expected 2 validation failures preserved, got %d", len(ctx.RecentValidationFailures))
	}
	if ctx.RecentValidationFailures[0] != "--- FAIL: TestFoo" {
		t.Errorf("expected first failure to be preserved, got %q", ctx.RecentValidationFailures[0])
	}

	// Other slice fields should also still be preserved
	if len(ctx.ConfirmedLearnings) != 1 {
		t.Errorf("expected 1 confirmed learning preserved, got %d", len(ctx.ConfirmedLearnings))
	}
	if len(ctx.RecentLearnings) != 1 {
		t.Errorf("expected 1 recent learning preserved, got %d", len(ctx.RecentLearnings))
	}
}
