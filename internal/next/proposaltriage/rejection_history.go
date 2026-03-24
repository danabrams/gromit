package proposaltriage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

// LoadRejectedProposals scans all runs for the given project, loads proposal-decisions.json files,
// collects decisions with Action="rejected", and returns a JSON array of objects with fields:
// type, title, proposed_change, rejection_reason.
func LoadRejectedProposals(storeDir, projectID string) (json.RawMessage, error) {
	store := runstore.NewStore(storeDir)

	// Get all runs for the project
	runs, err := store.List(projectID)
	if err != nil {
		return nil, err
	}

	type RejectedProposal struct {
		Type            string `json:"type"`
		Title           string `json:"title"`
		ProposedChange  string `json:"proposed_change"`
		RejectionReason string `json:"rejection_reason"`
	}

	var rejectedProposals []RejectedProposal

	// Process each run
	for _, run := range runs {
		// Load decisions from proposal-decisions.json for this run
		evidenceDir := store.RunEvidenceDir(run.RunID)
		decisions, err := LoadDecisions(evidenceDir)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// Skip runs with real errors (not just missing file)
			continue
		}

		// Load proposals from distillation-proposals.json; fall back to proposals.json.
		distProposalsPath := filepath.Join(evidenceDir, "distillation-proposals.json")
		if _, err := os.Stat(distProposalsPath); os.IsNotExist(err) {
			legacyPath := filepath.Join(evidenceDir, "proposals.json")
			if _, lerr := os.Stat(legacyPath); lerr == nil {
				distProposalsPath = legacyPath
			}
		}
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

		// Build proposal map by ID
		proposalMap := make(map[string]*reviewdistiller.Proposal)
		for i, prop := range distResult.Proposals {
			proposalMap[prop.ID] = &distResult.Proposals[i]
		}

		// Process rejected decisions
		for _, decision := range decisions {
			if decision.Action == "rejected" {
				if prop, ok := proposalMap[decision.ProposalID]; ok {
					rejectedProposals = append(rejectedProposals, RejectedProposal{
						Type:            prop.Type,
						Title:           prop.Title,
						ProposedChange:  prop.ProposedChange,
						RejectionReason: decision.Reason,
					})
				}
			}
		}
	}

	// Return as JSON
	result, err := json.Marshal(rejectedProposals)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(result), nil
}
