package escalation

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/analyzer"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// ExtractLearning saves a learning from failure analysis to the learnings file.
func ExtractLearning(bc *runtypes.BeadContext, analysis *analyzer.Analysis, lf *learnings.File) {
	if bc == nil || bc.Bead == nil {
		return
	}
	if analysis == nil || analysis.Learning == nil {
		return
	}
	if lf == nil {
		return
	}
	lf.Add(bc.Bead.ID, *analysis.Learning, analysis.LearningCategory())
}

// ExtractSyntheticLearning saves a synthetic learning from a custom message.
func ExtractSyntheticLearning(bc *runtypes.BeadContext, message string, lf *learnings.File) {
	if bc == nil || bc.Bead == nil {
		return
	}
	if lf == nil {
		return
	}
	lf.Add(bc.Bead.ID, message, "patterns")
}

// ExtractScopeTooLargeLearning saves a synthetic learning for scope-too-large failures.
func ExtractScopeTooLargeLearning(bc *runtypes.BeadContext, explanation string, lf *learnings.File) {
	learning := fmt.Sprintf("Bead '%s' was too large for %s — consider splitting beads with more than 3 acceptance criteria", bc.Bead.Title, bc.Model)
	ExtractSyntheticLearning(bc, learning, lf)
}

// ExtractTimeoutLearning saves a synthetic learning for timeout failures.
func ExtractTimeoutLearning(bc *runtypes.BeadContext, lf *learnings.File) {
	learning := fmt.Sprintf("Bead '%s' timed out on %s — may need simpler scope or higher model tier", bc.Bead.Title, bc.Model)
	ExtractSyntheticLearning(bc, learning, lf)
}

// SuccessLearningRouter is the narrow interface for provider selection during success learning extraction.
type SuccessLearningRouter interface {
	Select(phase, tier string) (SuccessLearningProvider, string)
}

// SuccessLearningProvider is the narrow interface for running a provider during success learning extraction.
type SuccessLearningProvider interface {
	Run(ctx context.Context, prompt string, tier string) (SuccessLearningResult, error)
}

// SuccessLearningResult is the narrow interface for provider result during success learning extraction.
type SuccessLearningResult interface {
	IsSuccess() bool
	GetOutput() string
}

// ExtractSuccessLearning calls Claude to extract a learning from a successful iteration.
// Skips extraction for low-tier beads.
func ExtractSuccessLearning(ctx context.Context, bc *runtypes.BeadContext, cfg *config.Config, lf *learnings.File, router interface{}, renderer interface{}) {
	if bc == nil {
		return
	}
	// Skip learning extraction for haiku-tier beads
	if bc.Tier == provider.TierLow {
		return
	}
	// Placeholder — full implementation requires router/renderer integration
	_ = cfg
	_ = lf
	_ = router
	_ = renderer
}
