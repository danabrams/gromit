package specloop

import (
	"context"

	"github.com/danabrams/gromit/internal/next/runstore"
)

// ActionKind indicates what the SpecLoop should do after a stage completes.
type ActionKind int

const (
	Continue ActionKind = iota
	ReplanFrom
	NeedsHuman
	Blocked
)

func (k ActionKind) String() string {
	switch k {
	case Continue:
		return "continue"
	case ReplanFrom:
		return "replan_from"
	case NeedsHuman:
		return "needs_human"
	case Blocked:
		return "blocked"
	default:
		return "unknown"
	}
}

// FailureContext carries details about why a stage returned a non-Continue action.
type FailureContext struct {
	Failures          []string `json:"failures"`
	Cycle             int      `json:"cycle"`
	Diff              string   `json:"diff,omitempty"`
	EscalatedFailures []string `json:"escalated_failures,omitempty"`
}

// See CLAUDE.md nil-field normalization visibility convention:
// exported — cross-package boundary type
// NormalizeNilFields maps nil slices to empty values for JSON consistency.
func (fc *FailureContext) NormalizeNilFields() {
	if fc.Failures == nil {
		fc.Failures = []string{}
	}
	if fc.EscalatedFailures == nil {
		fc.EscalatedFailures = []string{}
	}
}

// NextAction is returned by a Stage to tell the SpecLoop what to do next.
type NextAction struct {
	Kind    ActionKind
	Context *FailureContext
}

// Stage is a single phase of the spec execution loop.
type Stage interface {
	Name() string
	Run(ctx context.Context, run *runstore.RunState) (NextAction, error)
}
