package proposaltriage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// DiscoverPending lists all pending proposals (proposals without decisions) in runs.
// Takes optional filters for proposal types and run IDs.
// Results are sorted by creation time descending.
func DiscoverPending(rootDir, projectID string, proposalTypes *[]string, runIDs *[]string) ([]PendingProposal, error) {
	all, err := DiscoverAll(rootDir, projectID, proposalTypes, runIDs)
	if err != nil {
		return nil, err
	}

	var pending []PendingProposal
	for _, ap := range all {
		if ap.Decision == nil {
			pending = append(pending, PendingProposal{
				Proposal: ap.Proposal,
				RunID:    ap.RunID,
				SpecID:   ap.SpecID,
			})
		}
	}

	if pending == nil {
		pending = []PendingProposal{}
	}

	return pending, nil
}

// DiscoverAll lists all proposals (pending and decided) in runs.
// Takes optional filters for proposal types and run IDs.
// Results are sorted by creation time descending.
func DiscoverAll(rootDir, projectID string, proposalTypes *[]string, runIDs *[]string) ([]AllProposal, error) {
	store := runstore.NewStore(rootDir)

	// Get all runs for the project
	runs, err := store.List(projectID)
	if err != nil {
		return nil, err
	}

	// Build filter maps for efficient lookup
	typeFilter := make(map[string]bool)
	if proposalTypes != nil {
		for _, t := range *proposalTypes {
			typeFilter[t] = true
		}
	}

	runIDFilter := make(map[string]bool)
	if runIDs != nil {
		for _, id := range *runIDs {
			runIDFilter[id] = true
		}
	}

	var proposals []AllProposal
	createdTimeMap := make(map[string]time.Time) // Cache created times by runID

	// Process each run
	for _, run := range runs {
		// Apply runID filter if specified
		if len(runIDFilter) > 0 && !runIDFilter[run.RunID] {
			continue
		}

		// Load proposals from evidence directory
		evidenceDir := store.RunEvidenceDir(run.RunID)
		distProposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")

		data, err := os.ReadFile(distProposalsPath)
		if err != nil {
			// Skip runs without proposals
			continue
		}

		var distResult reviewdistiller.DistillationResult
		if err := json.Unmarshal(data, &distResult); err != nil {
			// Skip runs with invalid proposal JSON
			continue
		}

		// Cache the creation time for this run
		createdTimeMap[run.RunID] = distResult.CreatedAt

		// Load decisions from proposal-decisions.json for this run
		decisions, err := LoadDecisions(evidenceDir)
		if err != nil {
			// Skip if we can't load decisions
			continue
		}

		// Build decision map by proposal ID
		decisionMap := make(map[string]*Decision)
		for i, d := range decisions {
			d.NormalizeNilFields()
			decisionMap[d.ProposalID] = &decisions[i]
		}

		// Process each proposal
		for _, prop := range distResult.Proposals {
			// Apply proposal type filter if specified
			if len(typeFilter) > 0 && !typeFilter[prop.Type] {
				continue
			}

			ap := AllProposal{
				Proposal: &prop,
				RunID:    run.RunID,
				SpecID:   run.SpecID,
				Decision: decisionMap[prop.ID],
			}

			proposals = append(proposals, ap)
		}
	}

	// Sort by creation time descending
	sort.SliceStable(proposals, func(i, j int) bool {
		iTime := createdTimeMap[proposals[i].RunID]
		jTime := createdTimeMap[proposals[j].RunID]
		return iTime.After(jTime)
	})

	if proposals == nil {
		proposals = []AllProposal{}
	}

	return proposals, nil
}
