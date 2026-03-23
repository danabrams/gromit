package proposaltriage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// ClusterSemantically clusters ungrouped proposals using LLM semantic analysis.
// It invokes the LLM with a list of proposal summaries and returns clusters with descriptions.
// If LLM call fails, it returns all proposals as singletons with a non-nil error for logging.
func ClusterSemantically(ctx context.Context, proposals []PendingProposal, llm reviewdistiller.LLMCompleter) ([]ProposalGroup, error) {
	if len(proposals) == 0 {
		return []ProposalGroup{}, nil
	}

	// Filter out nil proposals
	var validProposals []PendingProposal
	for _, pp := range proposals {
		if pp.Proposal != nil {
			validProposals = append(validProposals, pp)
		}
	}

	if len(validProposals) == 0 {
		return []ProposalGroup{}, nil
	}

	// Guard against nil LLM
	if llm == nil {
		return makeSingletonGroups(validProposals), fmt.Errorf("LLM completer is nil")
	}

	// Build prompt for LLM clustering
	prompt := buildClusteringPrompt(validProposals)

	// Call LLM
	response, err := llm.Complete(ctx, prompt)
	if err != nil {
		// Degrade gracefully: return all as singletons
		return makeSingletonGroups(validProposals), err
	}

	// Parse LLM response
	clusters, err := parseClusteringResponse(response)
	if err != nil {
		// Degrade gracefully: return all as singletons
		return makeSingletonGroups(validProposals), err
	}

	// Build proposal groups from clusters
	return clustersToGroups(clusters, validProposals)
}

// buildClusteringPrompt creates a prompt for the LLM to cluster proposals.
func buildClusteringPrompt(proposals []PendingProposal) string {
	var proposalLines []string
	for _, pp := range proposals {
		if pp.Proposal == nil {
			continue
		}
		proposalLines = append(proposalLines, fmt.Sprintf(
			"- ID: %s, Type: %s, Title: %s, Change: %s",
			pp.Proposal.ID,
			pp.Proposal.Type,
			pp.Proposal.Title,
			pp.Proposal.ProposedChange,
		))
	}

	return fmt.Sprintf(`Analyze the following proposals and cluster semantically similar ones together.
Return a JSON response with the structure:
{
  "clusters": [
    {
      "proposal_ids": ["id1", "id2"],
      "description": "Human-readable description of why these are grouped"
    }
  ]
}

Proposals:
%s

Return ONLY valid JSON, no markdown formatting.`, strings.Join(proposalLines, "\n"))
}

// clusterResponse represents the parsed LLM clustering response.
type clusterResponse struct {
	Clusters []struct {
		ProposalIDs []string `json:"proposal_ids"`
		Description string   `json:"description"`
	} `json:"clusters"`
}

// parseClusteringResponse parses the LLM's JSON response.
func parseClusteringResponse(response string) (*clusterResponse, error) {
	var result clusterResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse clustering response: %w", err)
	}
	return &result, nil
}

// makeSingletonGroups creates one group per proposal (for failure cases).
func makeSingletonGroups(proposals []PendingProposal) []ProposalGroup {
	var groups []ProposalGroup
	for _, pp := range proposals {
		if pp.Proposal == nil {
			continue
		}
		groupID := fmt.Sprintf("singleton-%s", pp.Proposal.ID)
		groups = append(groups, ProposalGroup{
			GroupID:     groupID,
			Proposals:   []PendingProposal{pp},
			GroupReason: "singleton",
		})
	}
	return groups
}

// clustersToGroups converts LLM cluster response to ProposalGroup objects.
func clustersToGroups(clusters *clusterResponse, proposals []PendingProposal) ([]ProposalGroup, error) {
	// Build ID-to-proposal map for quick lookup
	idToProposal := make(map[string]PendingProposal)
	for _, pp := range proposals {
		if pp.Proposal != nil {
			idToProposal[pp.Proposal.ID] = pp
		}
	}

	// Track which proposals have been assigned to a cluster
	assigned := make(map[string]bool)

	var groups []ProposalGroup
	for _, cluster := range clusters.Clusters {
		// Sort proposal IDs to ensure deterministic GroupID regardless of LLM response order
		sort.Strings(cluster.ProposalIDs)

		var groupProposals []PendingProposal
		for _, proposalID := range cluster.ProposalIDs {
			if pp, exists := idToProposal[proposalID]; exists {
				groupProposals = append(groupProposals, pp)
				assigned[proposalID] = true
			}
		}

		if len(groupProposals) > 0 {
			// Generate consistent group ID from first sorted proposal ID's hash.
			// Use the first sorted proposal ID (not first in groupProposals) to ensure determinism
			// even if some proposals are missing from idToProposal.
			firstProposalID := cluster.ProposalIDs[0]
			groupID := fmt.Sprintf("cluster-%s", computeProposalHash(
				idToProposal[firstProposalID].Proposal.Type,
				idToProposal[firstProposalID].Proposal.ProposedChange,
			))

			groups = append(groups, ProposalGroup{
				GroupID:     groupID,
				Proposals:   groupProposals,
				GroupReason: cluster.Description,
			})
		}
	}

	// Any unassigned proposals become singletons
	for _, pp := range proposals {
		if pp.Proposal != nil && !assigned[pp.Proposal.ID] {
			groupID := fmt.Sprintf("singleton-%s", pp.Proposal.ID)
			groups = append(groups, ProposalGroup{
				GroupID:     groupID,
				Proposals:   []PendingProposal{pp},
				GroupReason: "singleton",
			})
		}
	}

	return groups, nil
}
