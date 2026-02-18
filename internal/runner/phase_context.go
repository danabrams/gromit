package runner

import (
	"context"
	"time"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

const (
	phaseTimeoutSourceOverride = "phase_override"
	phaseTimeoutSourceFallback = "bead_timeout"
)

// phaseContextMeta captures resolved timeout details for phase attribution.
type phaseContextMeta struct {
	Phase               string
	RequestedTimeout    time.Duration
	EffectiveTimeout    time.Duration
	ClampedByRunDeadline bool
	TimeoutSource       string
	ParentAlreadyCanceled bool
	NearRunDeadline     bool
}

func newPhaseContext(bc *runtypes.BeadContext, phase string, phaseTimeoutSeconds int) (context.Context, context.CancelFunc, phaseContextMeta) {
	parent := context.Background()
	parentCanceled := false
	if bc != nil && bc.ParentCtx != nil {
		parent = bc.ParentCtx
		parentCanceled = bc.ParentCtx.Err() != nil
	}

	requested := time.Duration(phaseTimeoutSeconds) * time.Second
	timeoutSource := phaseTimeoutSourceOverride
	if requested <= 0 && bc != nil {
		requested = bc.BeadTimeout
		timeoutSource = phaseTimeoutSourceFallback
	}
	if requested <= 0 {
		requested = time.Second
		timeoutSource = phaseTimeoutSourceFallback
	}

	effective := requested
	clamped := false
	nearRunDeadline := false
	if bc != nil && !bc.RunDeadline.IsZero() {
		remaining := time.Until(bc.RunDeadline)
		if remaining < effective {
			effective = remaining
			clamped = true
		}
	}
	if effective <= 0 {
		effective = time.Millisecond
		nearRunDeadline = true
	}
	meta := phaseContextMeta{
		Phase:                phase,
		RequestedTimeout:     requested,
		EffectiveTimeout:     effective,
		ClampedByRunDeadline: clamped,
		TimeoutSource:        timeoutSource,
		ParentAlreadyCanceled: parentCanceled,
		NearRunDeadline:      nearRunDeadline,
	}

	ctx, cancel := context.WithTimeout(parent, effective)
	return ctx, cancel, meta
}
