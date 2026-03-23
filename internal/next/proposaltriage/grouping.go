package proposaltriage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

// GroupByContentHash groups proposals by their content hash (type + proposed_change).
// Proposals with identical type and proposed_change (after whitespace normalization) are grouped together.
// Each group has GroupReason="exact_match" and a consistent GroupID based on content hash.
func GroupByContentHash(proposals []PendingProposal) []ProposalGroup {
	if len(proposals) == 0 {
		return []ProposalGroup{}
	}

	// Map from content hash to slice of proposals with that hash
	hashToProposals := make(map[string][]PendingProposal)
	// Track order of hashes seen
	var hashOrder []string

	for _, pp := range proposals {
		if pp.Proposal == nil {
			continue
		}

		// Compute content hash using same normalization as playbook.ComputeID
		hash := computeProposalHash(pp.Proposal.Type, pp.Proposal.ProposedChange)

		if _, exists := hashToProposals[hash]; !exists {
			hashOrder = append(hashOrder, hash)
		}
		hashToProposals[hash] = append(hashToProposals[hash], pp)
	}

	// Build result groups preserving order
	var result []ProposalGroup
	for _, hash := range hashOrder {
		group := ProposalGroup{
			GroupID:     hash,
			Proposals:   hashToProposals[hash],
			GroupReason: "exact_match",
		}
		result = append(result, group)
	}

	return result
}

// computeProposalHash generates a hash from type and proposed_change,
// using the same whitespace normalization as playbook.ComputeID.
func computeProposalHash(typ, proposedChange string) string {
	// Normalize whitespace: collapse multiple spaces/newlines/tabs to single space
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(proposedChange, " ")
	normalized = strings.TrimSpace(normalized)

	// Hash type and content together
	hashInput := fmt.Sprintf("%s:%s", typ, normalized)
	hash := sha256.Sum256([]byte(hashInput))

	// Return full hex string
	return fmt.Sprintf("%x", hash)
}

// GroupProposals runs the full grouping pipeline:
// 1. Groups proposals with identical type + proposed_change (exact matches)
// 2. Collects singleton proposals (not in multi-proposal exact match groups)
// 3. Passes singleton proposals to ClusterSemantically for semantic clustering
// 4. Merges results
// Returns groups and a warnings slice for LLM failures.
func GroupProposals(ctx context.Context, proposals []PendingProposal, llm reviewdistiller.LLMCompleter) ([]ProposalGroup, []string) {
	var warnings []string

	// Step 1: Group by content hash (exact matches)
	allHashGroups := GroupByContentHash(proposals)

	// Step 2: Collect singleton proposals (from single-proposal groups)
	// and keep only multi-proposal exact match groups
	var exactMatchGroups []ProposalGroup
	var singletons []PendingProposal

	for _, group := range allHashGroups {
		if len(group.Proposals) > 1 {
			// Multi-proposal group is an exact match
			exactMatchGroups = append(exactMatchGroups, group)
		} else {
			// Single-proposal group becomes a singleton for semantic clustering
			singletons = append(singletons, group.Proposals...)
		}
	}

	// Step 3: Cluster singletons semantically
	var semanticGroups []ProposalGroup
	if len(singletons) > 0 {
		if llm == nil {
			// If LLM is nil, make all singletons into singleton groups with warning
			for _, pp := range singletons {
				if pp.Proposal != nil {
					semanticGroups = append(semanticGroups, ProposalGroup{
						GroupID:     fmt.Sprintf("singleton-%s", pp.Proposal.ID),
						Proposals:   []PendingProposal{pp},
						GroupReason: "singleton",
					})
				}
			}
			warnings = append(warnings, "semantic clustering skipped: LLM completer is nil")
		} else {
			clustered, err := ClusterSemantically(ctx, singletons, llm)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("semantic clustering failed: %v", err))
			}
			semanticGroups = clustered
		}
	}

	// Step 4: Merge results
	allGroups := append(exactMatchGroups, semanticGroups...)

	return allGroups, warnings
}

// FilterGroupsByType filters proposal groups to only include groups where all proposals match the specified type.
func FilterGroupsByType(groups []ProposalGroup, proposalType string) []ProposalGroup {
	var filtered []ProposalGroup
	for _, group := range groups {
		// Check if all proposals in this group match the requested type
		allMatch := true
		for _, pp := range group.Proposals {
			if pp.Proposal == nil || pp.Proposal.Type != proposalType {
				allMatch = false
				break
			}
		}
		if allMatch && len(group.Proposals) > 0 {
			filtered = append(filtered, group)
		}
	}
	return filtered
}
