//go:build acceptance
// +build acceptance

package v2

import (
	"context"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/v2/stage"
	planstage "github.com/danabrams/gromit/internal/v2/stage/plan"
	"github.com/danabrams/gromit/internal/v2/stage/present"
)

func newAcceptancePlanStage(plan string) stage.Stage {
	return &acceptancePlanStage{plan: plan}
}

func newAcceptancePresentStage() (stage.Stage, *present.SummaryContext) {
	ctx := &present.SummaryContext{}
	return &acceptancePresentStage{}, ctx
}

type acceptancePlanStage struct {
	plan string
}

func (s *acceptancePlanStage) Name() string { return "plan" }

func (s *acceptancePlanStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	planPath := filepath.Join(req.Worktree, ".gromit", "v2", "plan.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(planPath, []byte(s.plan), 0o644); err != nil {
		return nil, err
	}
	return &stage.Result{
		Decision: stage.DecisionProceed,
		Artifacts: &planstage.PlanArtifacts{
			SpecID: req.Bead.ID,
			Plan:   s.plan,
			Path:   planPath,
			Model:  "acceptance-fake",
		},
	}, nil
}

type acceptancePresentStage struct{}

func (s *acceptancePresentStage) Name() string { return "present" }

func (s *acceptancePresentStage) Run(ctx context.Context, req *stage.Request) (*stage.Result, error) {
	return &stage.Result{Decision: stage.DecisionProceed}, nil
}
