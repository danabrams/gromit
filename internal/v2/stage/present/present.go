package present

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/presentation"
	v2review "github.com/danabrams/gromit/internal/v2/review"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
)

// SummaryContext captures the accumulated data the presentation stage surfaces.
type SummaryContext struct {
	Plan               string
	Worktree           string
	BeadSummaries      []presentation.BeadSummary
	AcceptanceResults  []presentation.AcceptanceResult
	OutOfScopeFindings []v2review.Finding
	BranchLink         string
	DiffLink           string
	Success            bool
	FailureSummary     string
	RemainingWork      []string
	IntegrationBranch  string
}

// Stage implements the presentation stage of the run loop.
type Stage struct {
	name      string
	presenter adapter.PresenterAdapter
	ctx       *SummaryContext
}

// New creates a present stage backed by the provided presenter and accumulated context.
func New(cfg *config.Config, presenter adapter.PresenterAdapter, ctx *SummaryContext) (*Stage, error) {
	if presenter == nil {
		return nil, errors.New("presenter required")
	}
	if ctx == nil {
		return nil, errors.New("summary context required")
	}
	return &Stage{
		name:      stagedesc.Describe("present", cfg),
		presenter: presenter,
		ctx:       ctx,
	}, nil
}

var _ stagepkg.Stage = (*Stage)(nil)

// Name returns the stage identifier consumed by the loop.
func (s *Stage) Name() string {
	return s.name
}

// Run builds the presentation summary and forwards it to the presenter.
func (s *Stage) Run(ctx context.Context, req *stagepkg.Request) (*stagepkg.Result, error) {
	if req == nil {
		return nil, fmt.Errorf("request required")
	}
	summary := s.buildPresentation(req)
	if err := s.presenter.PresentSummary(ctx, beadID(req), summary); err != nil {
		return nil, fmt.Errorf("present summary: %w", err)
	}
	return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
}

func (s *Stage) buildPresentation(req *stagepkg.Request) presentation.PresentationSummary {
	return presentation.PresentationSummary{
		SpecName:           beadID(req),
		SpecBranch:         presentation.SpecBranchName(beadID(req)),
		IntegrationBranch:  integrationBranch(req, s.ctx.IntegrationBranch),
		Plan:               s.ctx.Plan,
		Worktree:           s.ctx.Worktree,
		BeadSummaries:      cloneBeadSummaries(s.ctx.BeadSummaries),
		Success:            s.ctx.Success,
		AcceptanceResults:  cloneAcceptanceResults(s.ctx.AcceptanceResults),
		OutOfScopeFindings: cloneFindings(s.ctx.OutOfScopeFindings),
		FailureSummary:     s.ctx.FailureSummary,
		RemainingWork:      cloneStrings(s.ctx.RemainingWork),
		BranchLink:         trimLink(s.ctx.BranchLink),
		DiffLink:           trimLink(s.ctx.DiffLink),
	}
}

func beadID(req *stagepkg.Request) string {
	if req == nil {
		return ""
	}
	return req.Bead.ID
}

func integrationBranch(req *stagepkg.Request, override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	if req != nil && req.Config != nil {
		if trimmed := strings.TrimSpace(req.Config.Git.BaseBranch); trimmed != "" {
			return trimmed
		}
	}
	return presentation.DefaultIntegrationBranch()
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func cloneAcceptanceResults(src []presentation.AcceptanceResult) []presentation.AcceptanceResult {
	if len(src) == 0 {
		return nil
	}
	dst := make([]presentation.AcceptanceResult, len(src))
	copy(dst, src)
	return dst
}

func cloneFindings(src []v2review.Finding) []v2review.Finding {
	if len(src) == 0 {
		return nil
	}
	dst := make([]v2review.Finding, len(src))
	copy(dst, src)
	return dst
}

func cloneBeadSummaries(src []presentation.BeadSummary) []presentation.BeadSummary {
	if len(src) == 0 {
		return nil
	}
	dst := make([]presentation.BeadSummary, len(src))
	copy(dst, src)
	return dst
}

func trimLink(src string) string {
	return strings.TrimSpace(src)
}
