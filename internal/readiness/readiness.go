package readiness

import (
	"context"

	"github.com/danabrams/gromit/internal/bead"
)

// Status represents the result of a readiness assessment.
type Status string

const (
	// StatusReady indicates the bead is ready to proceed.
	StatusReady Status = "ready"
	// StatusNotReady indicates the bead should be blocked until readiness criteria are met.
	StatusNotReady Status = "not_ready"
)

// Assessment captures the outcome of a readiness assessment.
type Assessment struct {
	Status Status
	Reason string
}

// Assessor determines whether a bead is ready for execution.
type Assessor interface {
	Assess(ctx context.Context, b *bead.Bead) (Assessment, error)
}
