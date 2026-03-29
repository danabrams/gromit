package debug

import (
	"strings"
	"testing"
)

func TestDetectLearnablePattern(t *testing.T) {
	patterns := []LearningPattern{
		{
			ID: "boundary_missing",
			Trigger: func(ctx FailureContext) bool {
				return strings.Contains(ctx.Message, "boundary")
			},
		},
	}
	ctx := FailureContext{Message: "runtime boundary guard failure"}
	pattern := DetectLearnablePattern(ctx, patterns)
	if pattern == nil {
		t.Fatal("expected learnable pattern")
	}
	if pattern.ID != "boundary_missing" {
		t.Fatalf("pattern ID = %q, want %q", pattern.ID, "boundary_missing")
	}
}
