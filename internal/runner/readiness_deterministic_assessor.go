package runner

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/readiness"
)

type deterministicReadinessAssessor struct{}

func NewDeterministicReadinessAssessor() readiness.Assessor {
	return &deterministicReadinessAssessor{}
}

func (d *deterministicReadinessAssessor) Assess(ctx context.Context, b *bead.Bead) (readiness.Assessment, error) {
	if outcome, reason := prepare.CheckCriteriaPresence(b); outcome != prepare.ReadinessOutcomeReady {
		return readiness.Assessment{
			Status: readiness.StatusNotReady,
			Reason: reason,
		}, nil
	}
	return readiness.Assessment{Status: readiness.StatusReady}, nil
}
