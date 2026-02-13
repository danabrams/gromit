package escalation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

// successLearningResponse represents the JSON response from the learning extraction provider.
type successLearningResponse struct {
	Learning *string `json:"learning"`
	Category string  `json:"category"`
}

// ExtractSuccessLearning calls Claude to extract a learning from a successful iteration.
// Skips extraction for low-tier beads or when all touched packages have already been seen.
// The seenPackages map tracks packages processed in earlier iterations of the current run;
// if non-nil and all of bc.TouchedPackages are present, extraction is skipped.
// The router selects a provider to run the extraction;
// if router is nil or the provider is unavailable, extraction is silently skipped.
// The logFn callback is optional — if nil, logging is skipped.
func ExtractSuccessLearning(ctx context.Context, bc *runtypes.BeadContext, cfg *config.Config, lf *learnings.File, router SuccessLearningRouter, logFn LogFn, seenPackages map[string]bool) {
	if bc == nil {
		return
	}
	// Skip learning extraction for haiku-tier beads
	if bc.Tier == provider.TierLow {
		return
	}
	// Skip when all touched packages have been seen before in this run
	if seenPackages != nil && len(bc.TouchedPackages) > 0 {
		allSeen := true
		for _, pkg := range bc.TouchedPackages {
			if !seenPackages[pkg] {
				allSeen = false
				break
			}
		}
		if allSeen {
			return
		}
	}
	if router == nil {
		return
	}
	if lf == nil {
		return
	}

	p, tier := router.Select("build", provider.TierLow)
	if p == nil {
		return
	}

	// Build a simple prompt for learning extraction
	summary := bc.Bead.Title
	if bc.Bead.Description != "" {
		lines := strings.Split(bc.Bead.Description, "\n")
		if len(lines) > 0 && lines[0] != "" {
			summary = bc.Bead.Title + ": " + lines[0]
		}
	}
	prompt := fmt.Sprintf("Extract a learning from this successful bead: %s", summary)

	result, err := p.Run(ctx, prompt, tier)
	if err != nil {
		return
	}
	if result == nil || !result.IsSuccess() {
		return
	}

	output := result.GetOutput()
	var resp successLearningResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return
	}

	if resp.Learning == nil {
		return
	}

	category := resp.Category
	if category == "" {
		category = "patterns"
	}

	if logFn != nil {
		logFn("Success learning extracted: %s", *resp.Learning)
	}

	lf.Add(bc.Bead.ID, *resp.Learning, category)
}
