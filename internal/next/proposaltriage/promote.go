package proposaltriage

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
)

// Accept promotes a PendingProposal to a Decision, routing to the appropriate store
// (doctrine for doctrine_rule proposals, playbook for others).
// Field overrides (title, change, rationale) are used if non-empty; otherwise defaults from proposal are used.
// Before materializing, checks if an entry with the computed materialized ID already exists and is active.
// If a duplicate is found, skips materialization and sets DuplicateOf in the decision.
// Returns a Decision with materialized_id set based on proposal type and approved change text.
func Accept(
	pp *PendingProposal,
	overrideTitle string,
	overrideChange string,
	overrideRationale string,
	doctrineStore doctrine.Store,
	playbookStore *playbook.Store,
	doctrineDir string,
	playbookDir string,
	evidenceDir string,
) (*Decision, error) {
	if pp == nil || pp.Proposal == nil {
		return nil, fmt.Errorf("pending proposal is nil")
	}

	// Apply field overrides
	approvedTitle := overrideTitle
	if approvedTitle == "" {
		approvedTitle = pp.Proposal.Title
	}

	approvedChange := overrideChange
	if approvedChange == "" {
		approvedChange = pp.Proposal.ProposedChange
	}

	approvedRationale := overrideRationale
	if approvedRationale == "" {
		approvedRationale = pp.Proposal.Rationale
	}

	// Compute materialized ID based on proposal type
	var materializedID string
	var duplicateOf string
	switch pp.Proposal.Type {
	case "doctrine_rule":
		materializedID = computeDoctrineID(pp.Proposal.Type, approvedChange)
		// Check for duplicates in doctrine store
		if existingDoctrine, err := doctrineStore.Load(doctrineDir); err == nil {
			if isDuplicateInDoctrine(&existingDoctrine, materializedID) {
				duplicateOf = materializedID
			}
		}
		// Save to doctrine store only if not a duplicate
		if duplicateOf == "" {
			if err := promoteToDoctrine(pp, approvedTitle, approvedChange, approvedRationale, doctrineStore, doctrineDir); err != nil {
				return nil, fmt.Errorf("failed to promote to doctrine: %w", err)
			}
		}
	default:
		materializedID = computePlaybookID(pp.Proposal.Type, approvedChange)
		// Check for duplicates in playbook store
		if existingEntries, err := playbookStore.Load(); err == nil {
			if isDuplicateInPlaybook(existingEntries, materializedID) {
				duplicateOf = materializedID
			}
		}
		// Save to playbook store only if not a duplicate
		if duplicateOf == "" {
			if err := promoteToPlaybook(pp, approvedTitle, approvedChange, approvedRationale, playbookStore, playbookDir); err != nil {
				return nil, fmt.Errorf("failed to promote to playbook: %w", err)
			}
		}
	}

	decision := &Decision{
		ProposalID:        pp.Proposal.ID,
		Action:            "accepted",
		ApprovedTitle:     approvedTitle,
		ApprovedChange:    approvedChange,
		ApprovedRationale: approvedRationale,
		MaterializedID:    materializedID,
		DuplicateOf:       duplicateOf,
		DecidedAt:         time.Now(),
	}

	return decision, nil
}

// computeDoctrineID generates a deterministic ID for a doctrine rule.
// Uses SHA-256 hash of normalized type and change text, prefixes with "promoted-".
func computeDoctrineID(typ, change string) string {
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(change, " ")
	normalized = strings.TrimSpace(normalized)

	hashInput := fmt.Sprintf("%s:%s", typ, normalized)
	hash := sha256.Sum256([]byte(hashInput))
	hexStr := fmt.Sprintf("%x", hash)
	return "promoted-" + hexStr[:8]
}

// computePlaybookID generates a deterministic ID for a playbook entry.
// Uses the existing playbook.ComputeID logic.
func computePlaybookID(typ, change string) string {
	return playbook.ComputeID(typ, change)
}

// promoteToDoctrine saves the proposal as a doctrine rule.
func promoteToDoctrine(
	pp *PendingProposal,
	title string,
	change string,
	rationale string,
	store doctrine.Store,
	doctrineDir string,
) error {
	// Load existing doctrine
	existingDoctrine, err := store.Load(doctrineDir)
	if err != nil {
		return fmt.Errorf("failed to load doctrine: %w", err)
	}

	// Create a new rule
	rule := doctrine.NewRule(
		computeDoctrineID(pp.Proposal.Type, change),
		title,
		"*", // Scope defaults to all
	)

	// Set provenance fields for promoted rules
	rule.Source = fmt.Sprintf("promoted:%s", pp.Proposal.ID)
	rule.Status = "active"

	// Add the rule to existing doctrine
	existingDoctrine.Rules = append(existingDoctrine.Rules, rule)

	// Save back to store
	return store.Save(doctrineDir, existingDoctrine)
}

// promoteToPlaybook saves the proposal as a playbook entry.
func promoteToPlaybook(
	pp *PendingProposal,
	title string,
	change string,
	rationale string,
	store *playbook.Store,
	playbookDir string,
) error {
	// Load existing entries
	existingEntries, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load playbook entries: %w", err)
	}

	// Create a new entry
	entry := playbook.Entry{
		ID:               computePlaybookID(pp.Proposal.Type, change),
		Type:             pp.Proposal.Type,
		Title:            title,
		Content:          change,
		Rationale:        rationale,
		Status:           "active",
		SourceProposalID: pp.Proposal.ID,
		SourceRunID:      pp.RunID,
		SourceSpecID:     pp.SpecID,
		CreatedAt:        time.Now(),
	}

	// Add the entry to existing playbook
	existingEntries = append(existingEntries, entry)

	// Save back to store
	return store.Save(existingEntries)
}

// isDuplicateInDoctrine checks if a rule with the given ID already exists.
func isDuplicateInDoctrine(doc *doctrine.Doctrine, materializedID string) bool {
	if doc == nil {
		return false
	}
	for _, rule := range doc.Rules {
		if rule.ID == materializedID {
			return true
		}
	}
	return false
}

// isDuplicateInPlaybook checks if an entry with the given ID already exists and is active.
func isDuplicateInPlaybook(entries []playbook.Entry, materializedID string) bool {
	for _, entry := range entries {
		if entry.ID == materializedID && entry.Status == "active" {
			return true
		}
	}
	return false
}

// Reject creates a Decision with action=rejected for a PendingProposal.
// Records the rejection reason and current timestamp.
func Reject(pp *PendingProposal, reason string) (*Decision, error) {
	if pp == nil || pp.Proposal == nil {
		return nil, fmt.Errorf("pending proposal is nil")
	}

	decision := &Decision{
		ProposalID: pp.Proposal.ID,
		Action:     "rejected",
		Reason:     reason,
		DecidedAt:  time.Now(),
	}

	return decision, nil
}

// RejectAfterAccept supersedes a previously accepted entry and records the rejection.
// It looks up the entry by the materialized ID from the accepted decision,
// marks it as superseded with the rejection proposal ID, and saves both stores.
func RejectAfterAccept(
	acceptedDecision *Decision,
	rejectionDecision *Decision,
	doctrineStore doctrine.Store,
	playbookStore *playbook.Store,
	doctrineDir string,
	playbookDir string,
) error {
	if acceptedDecision == nil || rejectionDecision == nil {
		return fmt.Errorf("decisions cannot be nil")
	}

	materializedID := acceptedDecision.MaterializedID
	if materializedID == "" {
		return fmt.Errorf("accepted decision has no materialized ID")
	}

	// Determine which store to update based on materialized ID prefix
	if strings.HasPrefix(materializedID, "promoted-") {
		// Update doctrine store
		if doctrineStore == nil {
			return fmt.Errorf("doctrine store required for promoted entry")
		}

		existingDoctrine, err := doctrineStore.Load(doctrineDir)
		if err != nil {
			return fmt.Errorf("failed to load doctrine: %w", err)
		}

		// Find and update the rule
		found := false
		for i, rule := range existingDoctrine.Rules {
			if rule.ID == materializedID {
				existingDoctrine.Rules[i].Status = "superseded"
				existingDoctrine.Rules[i].SupersededBy = rejectionDecision.ProposalID
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("no doctrine rule found with ID %s", materializedID)
		}

		if err := doctrineStore.Save(doctrineDir, existingDoctrine); err != nil {
			return fmt.Errorf("failed to save doctrine: %w", err)
		}
	} else {
		// Update playbook store
		if playbookStore == nil {
			return fmt.Errorf("playbook store required for playbook entry")
		}

		existingEntries, err := playbookStore.Load()
		if err != nil {
			return fmt.Errorf("failed to load playbook entries: %w", err)
		}

		// Find and update the entry
		found := false
		for i, entry := range existingEntries {
			if entry.ID == materializedID {
				existingEntries[i].Status = "superseded"
				existingEntries[i].SupersededBy = rejectionDecision.ProposalID
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("no playbook entry found with ID %s", materializedID)
		}

		if err := playbookStore.Save(existingEntries); err != nil {
			return fmt.Errorf("failed to save playbook entries: %w", err)
		}
	}

	return nil
}
