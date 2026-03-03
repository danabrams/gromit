package runner

import (
	"io"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/runner/execution"
	"github.com/danabrams/gromit/internal/runner/pipeline/midreview"
)

func newMidReviewStage(renderer *prompt.Renderer, execInvoker *execution.Invoker, gitDiff midreview.GitDiffFn, output io.Writer, costDefs map[string]config.ProviderDef) pipeline.Stage {
	if renderer == nil || execInvoker == nil {
		return nil
	}
	return midreview.NewStage(
		renderer,
		&invokerAdapter{
			execInvoker:      execInvoker,
			providerCostDefs: costDefs,
		},
		gitDiff,
		output,
	)
}
