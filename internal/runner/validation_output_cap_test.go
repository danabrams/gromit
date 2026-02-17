package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// rendererFailBuild is a test renderer that always fails RenderBuild,
// allowing makeValidationExecuteFn to be exercised up to the render step.
type rendererFailBuild struct {
	mockRenderer
}

func (r *rendererFailBuild) RenderBuild(_ *prompt.Context) (string, error) {
	return "", fmt.Errorf("render error (intentional)")
}

// TestMakeValidationExecuteFn_TruncatesPrevFailure verifies that when
// makeValidationExecuteFn runs with a large Result.Output, the PrevFailure
// field assigned to PromptCtx is capped at ~50KB.
func TestMakeValidationExecuteFn_TruncatesPrevFailure(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	var logBuf strings.Builder
	r := &Runner{
		cfg:      cfg,
		renderer: &rendererFailBuild{},
		output:   &logBuf,
	}

	// Build Result.Output larger than 50KB
	line := strings.Repeat("y", 100) + "\n"
	var sb strings.Builder
	for sb.Len() < 60*1024 {
		sb.WriteString(line)
	}
	largeOutput := sb.String()

	bc := &runtypes.BeadContext{
		Bead:      &bead.Bead{ID: "test-cap", Title: "Cap test"},
		Result:    &runtypes.IterationResult{Output: largeOutput},
		PromptCtx: &prompt.Context{},
	}

	fn := r.makeValidationExecuteFn()
	fn(context.Background(), bc)

	const maxAllowed = 55 * 1024 // 50KB cap + small overhead for marker
	if len(bc.PromptCtx.PrevFailure) > maxAllowed {
		t.Errorf("PrevFailure length %d exceeds cap %d; large Result.Output not truncated before prompt context",
			len(bc.PromptCtx.PrevFailure), maxAllowed)
	}
}
