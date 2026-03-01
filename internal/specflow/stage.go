package specflow

import (
	"errors"
	"fmt"
)

type Stage string

const (
	StagePlanning        Stage = "planning"
	StageDrafting        Stage = "drafting" // kept for compatibility with legacy data
	StageAcceptanceTests Stage = "acceptance-tests"
	StageImplementation  Stage = "implementation"
	StageLocalGate       Stage = "local-gate"
	StageReview          Stage = "review"
	StageGlobalGate      Stage = "global-gate"
	StageDone            Stage = "done"
)

var ErrInvalidTransition = errors.New("invalid spec stage transition")

var stageTransitions = map[Stage]Stage{
	StagePlanning:        StageAcceptanceTests,
	StageAcceptanceTests: StageImplementation,
	StageImplementation:  StageLocalGate,
	StageLocalGate:       StageReview,
	StageReview:          StageGlobalGate,
	StageGlobalGate:      StageDone,
	StageDrafting:        StageReview,
}

func ValidateTransition(from, to Stage) error {
	expected, ok := stageTransitions[from]
	if !ok || expected != to {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, to)
	}
	return nil
}
