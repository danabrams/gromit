package specmerge_test

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/runner/specmerge"
)

func TestBuildPRSummaryIncludesSpecAndDiff(t *testing.T) {
	ctx := context.Background()
	input := specmerge.PRSummaryInput{
		SpecName:    "payments",
		SpecContent: "# Payments Spec\n- ensure cash flows",
		Diff:        "diff --git a/main.go b/main.go\n+func main() {}",
	}

	summary, err := specmerge.BuildPRSummary(ctx, input)
	if err != nil {
		t.Fatalf("BuildPRSummary returned error: %v", err)
	}

	if !strings.Contains(summary, "payments") {
		t.Fatalf("summary missing spec name: %s", summary)
	}
	if !strings.Contains(summary, "Spec excerpt") {
		t.Fatalf("summary missing spec section: %s", summary)
	}
	if !strings.Contains(summary, "Diff excerpt") {
		t.Fatalf("summary missing diff section: %s", summary)
	}
}
