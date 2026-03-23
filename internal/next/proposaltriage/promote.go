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

// SourceRunID string represents the run identifier from which a proposal originated.
type SourceRunID string

// Promote promotes a PendingProposal to a Decision, routing to the appropriate store
// (doctrine for doctrine_rule proposals, playbook for others).
// Field overrides (title, change, rationale) are used if non-empty; otherwise defaults from proposal are used.
// Before materializing, checks if an entry with the computed materialized ID already exists and is active.
// If a duplicate is found, skips materialization and sets DuplicateOf in the decision.
// Returns a Decision with materialized_id set based on proposal type and approved change text.
// scope ("local" or "global") is used to set the Scope field on materialized entries.
// Empty scope defaults to "*" for backward compatibility.
func Promote(
	pp *PendingProposal,
	overrideTitle string,
	overrideChange string,
	overrideRationale string,
	doctrineStore doctrine.Store,
	playbookStore *playbook.Store,
	scope string,
	evidenceDir string,
) (*Decision, error) {
	if pp == nil || pp.Proposal == nil {
		return nil, fmt.Errorf("pending proposal is nil")
	}

	// Validate scope: must be "local", "global", or empty (defaults to "*")
	if scope != "" && scope != "local" && scope != "global" {
		return nil, fmt.Errorf("invalid scope %q: must be 'local', 'global', or empty", scope)
	}

	// Validate that the proposal is not in a terminal state
	var decisions []Decision
	if evidenceDir != "" {
		var err error
		decisions, err = LoadDecisions(evidenceDir)
		if err != nil {
			return nil, fmt.Errorf("failed to load decisions: %w", err)
		}
	}
	if err := ValidateTerminalState(pp.Proposal.ID, decisions); err != nil {
		return nil, err
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
		if doctrineStore == nil {
			return nil, fmt.Errorf("doctrineStore is required for doctrine_rule proposals")
		}
		materializedID = computeDoctrineID(pp.Proposal.Type, approvedChange)
		// Check for duplicates in doctrine store
		if existingDoctrine, err := doctrineStore.Load(); err == nil {
			if isDuplicateInDoctrine(&existingDoctrine, materializedID) {
				duplicateOf = materializedID
			}
		}
		// Save to doctrine store only if not a duplicate
		if duplicateOf == "" {
			if err := promoteToDoctrine(pp, approvedTitle, approvedChange, doctrineStore, scope); err != nil {
				return nil, fmt.Errorf("failed to promote to doctrine: %w", err)
			}
		}
	default:
		if playbookStore == nil {
			return nil, fmt.Errorf("playbookStore is required for non-doctrine proposals")
		}
		materializedID = computePlaybookID(pp.Proposal.Type, approvedChange)
		// Check for duplicates in playbook store
		if existingEntries, err := playbookStore.Load(); err == nil {
			if isDuplicateInPlaybook(existingEntries, materializedID) {
				duplicateOf = materializedID
			}
		}
		// Save to playbook store only if not a duplicate
		if duplicateOf == "" {
			if err := promoteToPlaybook(pp, approvedTitle, approvedChange, approvedRationale, playbookStore, scope); err != nil {
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
// Note: rationale is intentionally not stored on doctrine.Rule — the Rule type
// captures the rule text (title/change) and provenance. The rationale from the
// original proposal is preserved in the Decision record, not duplicated here.
func promoteToDoctrine(
	pp *PendingProposal,
	title string,
	change string,
	store doctrine.Store,
	scope string,
) error {
	// Load existing doctrine
	existingDoctrine, err := store.Load()
	if err != nil {
		return fmt.Errorf("failed to load doctrine: %w", err)
	}

	// Default scope to "*" if empty (backward compatible)
	ruleScope := scope
	if ruleScope == "" {
		ruleScope = "*"
	}

	// Create a new rule
	rule := doctrine.NewRule(
		computeDoctrineID(pp.Proposal.Type, change),
		title,
		ruleScope,
	)

	// Set provenance fields for promoted rules
	rule.Source = fmt.Sprintf("promoted:%s", pp.Proposal.ID)
	rule.Status = "active"
	rule.SourceProposalID = pp.Proposal.ID
	rule.SourceRunID = pp.RunID
	rule.SourceSpecID = pp.SpecID

	// Add the rule to existing doctrine
	existingDoctrine.Rules = append(existingDoctrine.Rules, rule)

	// Save back to store
	return store.Save(existingDoctrine)
}

// promoteToPlaybook saves the proposal as a playbook entry.
func promoteToPlaybook(
	pp *PendingProposal,
	title string,
	change string,
	rationale string,
	store *playbook.Store,
	scope string,
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
		Scope:            scope,
	}

	// Add the entry to existing playbook
	existingEntries = append(existingEntries, entry)

	// Save back to store
	return store.Save(existingEntries)
}

// isDuplicateInDoctrine checks if a rule with the given ID already exists and is active.
func isDuplicateInDoctrine(doc *doctrine.Doctrine, materializedID string) bool {
	if doc == nil {
		return false
	}
	for _, rule := range doc.Rules {
		if rule.ID == materializedID && rule.Status == "active" {
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
