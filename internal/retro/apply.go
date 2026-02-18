package retro

import (
	"fmt"

	"github.com/danabrams/gromit/internal/learnings"
)

// ApplyProposals applies retro proposals to learnings and rules files.
func ApplyProposals(proposals *Proposals, lf *learnings.File, rulesPath string) error {
	if proposals == nil {
		return nil
	}
	if lf == nil {
		return fmt.Errorf("learnings file is nil")
	}

	for _, consolidation := range proposals.Consolidations {
		if err := lf.Replace(consolidation.LearningHashes, consolidation.ConsolidatedText, learnings.CategoryPatterns); err != nil {
			return err
		}
	}

	return nil
}
