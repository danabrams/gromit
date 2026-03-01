package readiness

import (
	"context"
	"strings"

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

const (
	// ReadinessOverrideReasonPrefix tags reasons that occurred during an override bypass.
	ReadinessOverrideReasonPrefix = "readiness_override:"
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

// NormalizeReason removes any override marker from the readiness reason and reports whether it was present.
func NormalizeReason(reason string) (string, bool) {
	if reason == "" {
		return "", false
	}
	if strings.HasPrefix(reason, ReadinessOverrideReasonPrefix) {
		return strings.TrimPrefix(reason, ReadinessOverrideReasonPrefix), true
	}
	return reason, false
}
