package midreview

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/provider"
)

// Stage implements the mid-review pipeline stage that runs between build and validate.
type Stage struct {
	events.EmitterMixin
	renderer PromptRenderer
	invoker  Invoker
	gitDiff  GitDiffFn
	output   io.Writer
}

// PromptRenderer renders the mid-build review prompt.
type PromptRenderer interface {
	RenderMidBuildReview(ctx *MidBuildReviewContext) (string, error)
}

// MidBuildReviewContext represents the data required to render the mid-review prompt.
type MidBuildReviewContext struct {
	BeadTitle          string
	BeadDescription    string
	Diff               string
	AcceptanceCriteria string
	Spec               string
}

func (s *Stage) invocationContext(ctx context.Context, in pipeline.Input) (context.Context, context.CancelFunc) {
	cfg := in.Config
	var deadline time.Time
	if !in.Deadline.IsZero() {
		deadline = in.Deadline
	}
	if cfg != nil {
		timeout := time.Duration(cfg.MidBuildReview.Timeout)
		if timeout > 0 {
			timeoutDeadline := time.Now().Add(timeout)
			if deadline.IsZero() || timeoutDeadline.Before(deadline) {
				deadline = timeoutDeadline
			}
		}
	}
	if deadline.IsZero() {
		return ctx, nil
	}
	return context.WithDeadline(ctx, deadline)
}

func parseMidReviewResult(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	var findings []string
	if err := json.Unmarshal([]byte(raw), &findings); err != nil {
		return nil, err
	}
	return findings, nil
}

// Invoker executes provider invocations for mid-review.
type Invoker interface {
	StreamRun(ctx context.Context, prompt, tier string, w io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
}

// GitDiffFn returns the current git diff used by the mid-review prompt.
type GitDiffFn func(ctx context.Context) (string, error)

// NewStage creates a new mid-review Stage.
func NewStage(renderer PromptRenderer, invoker Invoker, gitDiff GitDiffFn, output io.Writer) *Stage {
	return &Stage{
		renderer: renderer,
		invoker:  invoker,
		gitDiff:  gitDiff,
		output:   output,
	}
}

// Run executes the mid-review stage. It currently only checks whether the stage is enabled.
func (s *Stage) Run(ctx context.Context, in pipeline.Input) (pipeline.Output, error) {
	cfg := in.Config
	if cfg == nil || !cfg.MidBuildReview.Enabled {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}
	if s == nil || s.gitDiff == nil || s.renderer == nil || s.invoker == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	diff, err := s.gitDiff(ctx)
	if err != nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	contextData := &MidBuildReviewContext{
		Diff: diff,
	}
	if in.Bead != nil {
		contextData.BeadTitle = in.Bead.Title
		contextData.BeadDescription = in.Bead.Description
		contextData.AcceptanceCriteria = in.Bead.AcceptanceCriteria
	}
	promptText, err := s.renderer.RenderMidBuildReview(contextData)
	if err != nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	tier := strings.TrimSpace(cfg.MidBuildReview.Tier)
	if tier == "" {
		tier = provider.TierMedium
	}

	invocationCtx, cancel := s.invocationContext(ctx, in)
	if cancel != nil {
		defer cancel()
	}

	outputWriter := s.output
	if outputWriter == nil {
		outputWriter = io.Discard
	}

	result, err := s.invoker.StreamRun(invocationCtx, promptText, tier, outputWriter, nil, nil)
	if err != nil || result == nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	findings, err := parseMidReviewResult(result.Output)
	if err != nil {
		return pipeline.Output{Decision: pipeline.Proceed}, nil
	}

	return pipeline.Output{
		Decision:               pipeline.Proceed,
		MidBuildReviewFindings: findings,
		DurationMs:             result.Duration.Milliseconds(),
		CostUSD:                result.CostUSD,
		InputTokens:            result.InputTokens,
		OutputTokens:           result.OutputTokens,
	}, nil
}
