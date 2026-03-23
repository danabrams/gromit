package proposaltriage

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
)

func TestReject_CreatesRejectionDecision(t *testing.T) {
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:    "prop-123",
			Title: "Test Proposal",
		},
		RunID:  "run-456",
		SpecID: "spec-789",
	}

	decision, err := Reject(pp, "not ready for promotion", nil)

	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Reject returned nil decision")
	}

	if decision.Action != "rejected" {
		t.Errorf("Action = %q, want %q", decision.Action, "rejected")
	}

	if decision.ProposalID != "prop-123" {
		t.Errorf("ProposalID = %q, want %q", decision.ProposalID, "prop-123")
	}

	if decision.Reason != "not ready for promotion" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "not ready for promotion")
	}

	if decision.DecidedAt.IsZero() {
		t.Error("DecidedAt should not be zero")
	}
}

func TestReject_NilProposalReturnsError(t *testing.T) {
	_, err := Reject(nil, "some reason", nil)

	if err == nil {
		t.Fatal("Reject should error on nil proposal")
	}
}

func TestReject_NilInternalProposalReturnsError(t *testing.T) {
	pp := &PendingProposal{
		Proposal: nil,
	}

	_, err := Reject(pp, "some reason", nil)

	if err == nil {
		t.Fatal("Reject should error on nil internal proposal")
	}
}

func TestReject_RecordsReason(t *testing.T) {
	tmpDir := t.TempDir()

	// Create initial stores with content
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir
	initialDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:      "initial-rule-1",
				Summary: "Initial Rule",
				Status:  "active",
			},
		},
	}
	if err := docStore.Save(initialDoctrine); err != nil {
		t.Fatalf("Failed to save initial doctrine: %v", err)
	}

	pbStore := &playbook.Store{Dir: tmpDir}
	initialEntries := []playbook.Entry{
		{
			ID:     "initial-entry-1",
			Status: "active",
			Title:  "Initial Entry",
		},
	}
	if err := pbStore.Save(initialEntries); err != nil {
		t.Fatalf("Failed to save initial playbook entries: %v", err)
	}

	// Call Reject with a reason
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:    "prop-999",
			Title: "Test Proposal",
		},
		RunID:  "run-999",
		SpecID: "spec-999",
	}

	testReason := "insufficient evidence for change"
	decision, err := Reject(pp, testReason, nil)

	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// Verify decision records the reason
	if decision.Reason != testReason {
		t.Errorf("Reason = %q, want %q", decision.Reason, testReason)
	}

	if decision.Action != "rejected" {
		t.Errorf("Action = %q, want %q", decision.Action, "rejected")
	}

	if decision.ProposalID != "prop-999" {
		t.Errorf("ProposalID = %q, want %q", decision.ProposalID, "prop-999")
	}

	// Verify doctrine store is NOT modified
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Errorf("Doctrine rules count = %d, want 1", len(loadedDoctrine.Rules))
	}

	if len(loadedDoctrine.Rules) > 0 && loadedDoctrine.Rules[0].ID != "initial-rule-1" {
		t.Errorf("Doctrine rule ID changed, want initial-rule-1")
	}

	// Verify playbook store is NOT modified
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Errorf("Playbook entries count = %d, want 1", len(loadedEntries))
	}

	if len(loadedEntries) > 0 && loadedEntries[0].ID != "initial-entry-1" {
		t.Errorf("Playbook entry ID changed, want initial-entry-1")
	}
}

func TestRejectAfterAccept_SupersededPlaybookEntry(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an accepted decision with materialized ID for a playbook entry
	acceptedDecision := &Decision{
		ProposalID:     "prop-123",
		Action:         "accepted",
		MaterializedID: "pb-abc12345",
	}

	// Create an existing playbook entry with that ID
	pbStore := &playbook.Store{Dir: tmpDir}
	existingEntries := []playbook.Entry{
		{
			ID:     "pb-abc12345",
			Status: "active",
			Title:  "Original Rule",
		},
	}
	if err := pbStore.Save(existingEntries); err != nil {
		t.Fatalf("Failed to save playbook entries: %v", err)
	}

	// Create rejection decision
	rejectionDecision := &Decision{
		ProposalID: "prop-456",
		Action:     "rejected",
		Reason:     "better approach found",
	}

	// Call RejectAfterAccept
	err := RejectAfterAccept(
		acceptedDecision,
		rejectionDecision,
		[]Decision{*acceptedDecision},
		nil, // doctrineStore not needed for playbook
		pbStore,
	)

	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	// Verify playbook entry is superseded
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]
	if entry.Status != "superseded" {
		t.Errorf("Status = %q, want %q", entry.Status, "superseded")
	}

	if entry.SupersededBy != "prop-456" {
		t.Errorf("SupersededBy = %q, want %q", entry.SupersededBy, "prop-456")
	}
}

func TestRejectAfterAccept_SupersededDoctrineRule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an accepted decision with materialized ID for a doctrine rule
	acceptedDecision := &Decision{
		ProposalID:     "prop-123",
		Action:         "accepted",
		MaterializedID: "promoted-abc12345",
	}

	// Create existing doctrine with that rule
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir
	existingDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:      "promoted-abc12345",
				Summary: "Original Rule",
				Status:  "active",
			},
		},
	}
	if err := docStore.Save(existingDoctrine); err != nil {
		t.Fatalf("Failed to save doctrine: %v", err)
	}

	// Create rejection decision
	rejectionDecision := &Decision{
		ProposalID: "prop-456",
		Action:     "rejected",
		Reason:     "better approach found",
	}

	// Call RejectAfterAccept
	err := RejectAfterAccept(
		acceptedDecision,
		rejectionDecision,
		[]Decision{*acceptedDecision},
		docStore,
		nil, // playbookStore not needed for doctrine
	)

	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	// Verify doctrine rule is superseded
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]
	if rule.Status != "superseded" {
		t.Errorf("Status = %q, want %q", rule.Status, "superseded")
	}

	if rule.SupersededBy != "prop-456" {
		t.Errorf("SupersededBy = %q, want %q", rule.SupersededBy, "prop-456")
	}
}

func TestAccept_DoctrineRule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a pending proposal with type "doctrine_rule"
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-123",
			Type:           "doctrine_rule",
			Title:          "Always use descriptive variable names",
			ProposedChange: "Enforce clear naming conventions in all code",
			Rationale:      "Improves code readability and maintainability",
		},
		RunID:  "run-456",
		SpecID: "spec-789",
	}

	// Create doctrine store
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir

	// Call Promote with doctrine_rule proposal
	decision, err := Promote(
		pp,
		"", // no title override
		"", // no change override
		"", // no rationale override
		docStore,
		nil,     // playbookStore not needed for doctrine_rule
		"local", // use local scope
		tmpDir,
	)

	// Verify decision was created successfully
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify decision fields
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	if decision.ProposalID != "prop-123" {
		t.Errorf("ProposalID = %q, want %q", decision.ProposalID, "prop-123")
	}

	if decision.MaterializedID == "" || decision.DuplicateOf != "" {
		t.Fatal("MaterializedID should be set and DuplicateOf should be empty for new proposal")
	}

	// Verify MaterializedID has correct format (promoted-<hash>)
	if !strings.HasPrefix(decision.MaterializedID, "promoted-") {
		t.Errorf("MaterializedID = %q, want prefix 'promoted-'", decision.MaterializedID)
	}

	if decision.DecidedAt.IsZero() {
		t.Error("DecidedAt should not be zero")
	}

	// Load doctrine and verify rule was materialized
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// Verify rule ID matches decision's materialized ID
	if rule.ID != decision.MaterializedID {
		t.Errorf("Rule ID = %q, want %q", rule.ID, decision.MaterializedID)
	}

	// Verify rule summary comes from proposal title
	if rule.Summary != "Always use descriptive variable names" {
		t.Errorf("Summary = %q, want %q", rule.Summary, "Always use descriptive variable names")
	}

	// Verify rule source is set to promoted:<proposal-id>
	expectedSource := "promoted:prop-123"
	if rule.Source != expectedSource {
		t.Errorf("Source = %q, want %q", rule.Source, expectedSource)
	}

	// Verify rule status is active
	if rule.Status != "active" {
		t.Errorf("Status = %q, want %q", rule.Status, "active")
	}

	// Verify provenance fields are set
	if rule.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestAccept_PlannerHeuristic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a pending proposal with type "planner_heuristic"
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-heur-001",
			Type:           "planner_heuristic",
			Title:          "Cache query results",
			ProposedChange: "Add caching layer for repeated queries to improve performance",
			Rationale:      "Reduces database load and improves response times",
		},
		RunID:  "run-xyz-789",
		SpecID: "spec-cache-001",
	}

	// Create playbook store
	pbStore := &playbook.Store{Dir: tmpDir}

	// Call Promote with planner_heuristic proposal
	decision, err := Promote(
		pp,
		"",  // no title override
		"",  // no change override
		"",  // no rationale override
		nil, // doctrineStore not needed for planner_heuristic
		pbStore,
		"local", // use local scope
		tmpDir,
	)

	// Verify decision was created successfully
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify decision fields
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	if decision.ProposalID != "prop-heur-001" {
		t.Errorf("ProposalID = %q, want %q", decision.ProposalID, "prop-heur-001")
	}

	if decision.MaterializedID == "" || decision.DuplicateOf != "" {
		t.Fatal("MaterializedID should be set and DuplicateOf should be empty for new proposal")
	}

	// Verify MaterializedID has correct format (pb-<hash>)
	if !strings.HasPrefix(decision.MaterializedID, "pb-") {
		t.Errorf("MaterializedID = %q, want prefix 'pb-'", decision.MaterializedID)
	}

	if decision.DecidedAt.IsZero() {
		t.Error("DecidedAt should not be zero")
	}

	// Load playbook entries and verify entry was materialized
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]

	// Verify entry ID matches decision's materialized ID
	if entry.ID != decision.MaterializedID {
		t.Errorf("Entry ID = %q, want %q", entry.ID, decision.MaterializedID)
	}

	// Verify entry type is planner_heuristic
	if entry.Type != "planner_heuristic" {
		t.Errorf("Type = %q, want %q", entry.Type, "planner_heuristic")
	}

	// Verify entry status is active
	if entry.Status != "active" {
		t.Errorf("Status = %q, want %q", entry.Status, "active")
	}

	// Verify content comes from proposed_change
	expectedContent := "Add caching layer for repeated queries to improve performance"
	if entry.Content != expectedContent {
		t.Errorf("Content = %q, want %q", entry.Content, expectedContent)
	}

	// Verify provenance fields are set
	if entry.SourceProposalID != "prop-heur-001" {
		t.Errorf("SourceProposalID = %q, want %q", entry.SourceProposalID, "prop-heur-001")
	}

	if entry.SourceRunID != "run-xyz-789" {
		t.Errorf("SourceRunID = %q, want %q", entry.SourceRunID, "run-xyz-789")
	}

	if entry.SourceSpecID != "spec-cache-001" {
		t.Errorf("SourceSpecID = %q, want %q", entry.SourceSpecID, "spec-cache-001")
	}

	if entry.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestRejectAfterAccept_UnknownMaterializedIDReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	pbStore := &playbook.Store{Dir: tmpDir}

	acceptedDecision := &Decision{
		ProposalID:     "prop-123",
		Action:         "accepted",
		MaterializedID: "pb-nonexistent",
	}

	rejectionDecision := &Decision{
		ProposalID: "prop-456",
		Action:     "rejected",
		Reason:     "found issue",
	}

	err := RejectAfterAccept(
		acceptedDecision,
		rejectionDecision,
		[]Decision{},
		nil,
		pbStore,
	)

	if err == nil {
		t.Fatal("RejectAfterAccept should error when materialized entry not found")
	}
}

func TestAccept_FieldOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a pending proposal with original values
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-001",
			Type:           "design_principle",
			Title:          "Original Title",
			ProposedChange: "Original change description",
			Rationale:      "Original rationale for the change",
		},
		RunID:  "run-111",
		SpecID: "spec-222",
	}

	// Create playbook store
	pbStore := &playbook.Store{Dir: tmpDir}

	// Call Promote with field overrides
	decision, err := Promote(
		pp,
		"Overridden Title",       // override title
		"Overridden change text", // override change
		"Overridden rationale",   // override rationale
		nil,                      // doctrineStore not needed
		pbStore,
		"local", // use local scope
		tmpDir,
	)

	// Verify decision was created successfully
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify decision action
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify decision records the overridden values
	if decision.ApprovedTitle != "Overridden Title" {
		t.Errorf("ApprovedTitle = %q, want %q", decision.ApprovedTitle, "Overridden Title")
	}

	if decision.ApprovedChange != "Overridden change text" {
		t.Errorf("ApprovedChange = %q, want %q", decision.ApprovedChange, "Overridden change text")
	}

	if decision.ApprovedRationale != "Overridden rationale" {
		t.Errorf("ApprovedRationale = %q, want %q", decision.ApprovedRationale, "Overridden rationale")
	}

	if decision.MaterializedID == "" {
		t.Fatal("MaterializedID should be set")
	}

	// Load playbook and verify materialized entry uses overridden values
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]

	// Verify entry ID matches decision's materialized ID
	if entry.ID != decision.MaterializedID {
		t.Errorf("Entry ID = %q, want %q", entry.ID, decision.MaterializedID)
	}

	// Verify materialized entry uses overridden values, not original values
	if entry.Title != "Overridden Title" {
		t.Errorf("Entry Title = %q, want %q", entry.Title, "Overridden Title")
	}

	if entry.Content != "Overridden change text" {
		t.Errorf("Entry Content = %q, want %q", entry.Content, "Overridden change text")
	}

	if entry.Rationale != "Overridden rationale" {
		t.Errorf("Entry Rationale = %q, want %q", entry.Rationale, "Overridden rationale")
	}

	// Verify entry status is active
	if entry.Status != "active" {
		t.Errorf("Status = %q, want %q", entry.Status, "active")
	}

	// Verify provenance fields
	if entry.SourceProposalID != "prop-001" {
		t.Errorf("SourceProposalID = %q, want %q", entry.SourceProposalID, "prop-001")
	}
}

func TestAccept_DuplicateDetection_Doctrine(t *testing.T) {
	tmpDir := t.TempDir()

	// Define a proposal with specific change text
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dup-001",
			Type:           "doctrine_rule",
			Title:          "Never use global variables",
			ProposedChange: "Ban global variables in all code",
			Rationale:      "Improves code maintainability",
		},
		RunID:  "run-dup-001",
		SpecID: "spec-dup-001",
	}

	// Pre-compute the materialized ID using the same logic as computeDoctrineID
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(pp.Proposal.ProposedChange, " ")
	normalized = strings.TrimSpace(normalized)
	hashInput := fmt.Sprintf("%s:%s", pp.Proposal.Type, normalized)
	hash := sha256.Sum256([]byte(hashInput))
	hexStr := fmt.Sprintf("%x", hash)
	expectedMaterializedID := "promoted-" + hexStr[:8]

	// Create doctrine store with an existing active rule with the same ID
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir
	existingDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:      expectedMaterializedID,
				Summary: "Existing Rule with Same ID",
				Status:  "active",
			},
		},
	}
	if err := docStore.Save(existingDoctrine); err != nil {
		t.Fatalf("Failed to save doctrine: %v", err)
	}

	// Call Promote with a proposal that would compute to the same ID
	decision, err := Promote(
		pp,
		"", // no title override
		"", // no change override
		"", // no rationale override
		docStore,
		nil,     // playbookStore not needed
		"local", // use local scope
		"",      // evidenceDir
	)

	// Verify decision was created successfully
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify action is accepted
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify MaterializedID matches the expected ID
	if decision.MaterializedID != expectedMaterializedID {
		t.Errorf("MaterializedID = %q, want %q", decision.MaterializedID, expectedMaterializedID)
	}

	// Verify DuplicateOf is set to the existing entry ID (indicating duplicate was detected)
	if decision.DuplicateOf != expectedMaterializedID {
		t.Errorf("DuplicateOf = %q, want %q", decision.DuplicateOf, expectedMaterializedID)
	}

	// Verify that no new rule was created (should still have only 1 rule)
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d (should not have created duplicate)", len(loadedDoctrine.Rules))
	}

	// Verify the original rule is still active (unchanged)
	rule := loadedDoctrine.Rules[0]
	if rule.Status != "active" {
		t.Errorf("Rule status = %q, want %q", rule.Status, "active")
	}

	if rule.Summary != "Existing Rule with Same ID" {
		t.Errorf("Rule summary = %q, want %q", rule.Summary, "Existing Rule with Same ID")
	}
}

func TestAccept_DuplicateDetection_Playbook(t *testing.T) {
	tmpDir := t.TempDir()

	// Define a proposal with specific change text
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dup-pb-001",
			Type:           "planner_heuristic",
			Title:          "Cache query results",
			ProposedChange: "Add caching layer for repeated database queries",
			Rationale:      "Reduces database load",
		},
		RunID:  "run-dup-pb-001",
		SpecID: "spec-dup-pb-001",
	}

	// Create playbook store with an existing active entry with a specific ID
	pbStore := &playbook.Store{Dir: tmpDir}
	existingEntries := []playbook.Entry{
		{
			ID:     "pb-cache-layer",
			Type:   "planner_heuristic",
			Title:  "Existing Cache Entry",
			Status: "active",
		},
	}
	if err := pbStore.Save(existingEntries); err != nil {
		t.Fatalf("Failed to save playbook entries: %v", err)
	}

	// Create a proposal with change text that computes to the same ID
	// We need to figure out what change text produces "pb-cache-layer"
	// For simplicity, we'll just verify that if we accept with a known change text,
	// it produces a specific ID, then pre-seed the store with that ID

	// Alternative: Call Accept once to get the ID, then re-test with a pre-seeded store
	// But for this test, let's use a simpler approach: pre-seed with any ID and then
	// verify duplicate detection works when the computed ID matches pre-seeded one.

	// Actually, let's compute what the ID would be for our proposal
	expectedMaterializedID := playbook.ComputeID(pp.Proposal.Type, pp.Proposal.ProposedChange)

	// Clear the store and re-populate with the expected ID
	existingEntries = []playbook.Entry{
		{
			ID:     expectedMaterializedID,
			Type:   pp.Proposal.Type,
			Title:  "Existing Entry with Same ID",
			Status: "active",
		},
	}
	if err := pbStore.Save(existingEntries); err != nil {
		t.Fatalf("Failed to save playbook entries: %v", err)
	}

	// Call Promote with the proposal that would compute to the same ID
	decision, err := Promote(
		pp,
		"",  // no title override
		"",  // no change override
		"",  // no rationale override
		nil, // doctrineStore not needed
		pbStore,
		"local", // use local scope
		"",      // evidenceDir
	)

	// Verify decision was created successfully
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify action is accepted
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify MaterializedID matches the expected ID
	if decision.MaterializedID != expectedMaterializedID {
		t.Errorf("MaterializedID = %q, want %q", decision.MaterializedID, expectedMaterializedID)
	}

	// Verify DuplicateOf is set to the existing entry ID (indicating duplicate was detected)
	if decision.DuplicateOf != expectedMaterializedID {
		t.Errorf("DuplicateOf = %q, want %q", decision.DuplicateOf, expectedMaterializedID)
	}

	// Verify that no new entry was created (should still have only 1 entry)
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d (should not have created duplicate)", len(loadedEntries))
	}

	// Verify the original entry is still active (unchanged)
	entry := loadedEntries[0]
	if entry.Status != "active" {
		t.Errorf("Entry status = %q, want %q", entry.Status, "active")
	}

	if entry.Title != "Existing Entry with Same ID" {
		t.Errorf("Entry title = %q, want %q", entry.Title, "Existing Entry with Same ID")
	}
}

func TestRejectAfterAccept_Supersedes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an accepted decision with materialized ID
	acceptedDecision := &Decision{
		ProposalID:     "prop-123",
		Action:         "accepted",
		MaterializedID: "pb-abc12345",
		Reason:         "initial acceptance",
	}

	// Create an existing playbook entry with that ID
	pbStore := &playbook.Store{Dir: tmpDir}
	existingEntries := []playbook.Entry{
		{
			ID:     "pb-abc12345",
			Status: "active",
			Title:  "Original Heuristic",
		},
	}
	if err := pbStore.Save(existingEntries); err != nil {
		t.Fatalf("Failed to save playbook entries: %v", err)
	}

	// Create rejection decision that will supersede the accepted one
	rejectionDecision := &Decision{
		ProposalID: "prop-456",
		Action:     "rejected",
		Reason:     "better approach found",
	}

	// Call RejectAfterAccept
	err := RejectAfterAccept(
		acceptedDecision,
		rejectionDecision,
		[]Decision{*acceptedDecision},
		nil, // doctrineStore not needed for playbook
		pbStore,
	)

	if err != nil {
		t.Fatalf("RejectAfterAccept failed: %v", err)
	}

	// Verify that the materialized entry is superseded
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]

	// Verify entry status is set to superseded
	if entry.Status != "superseded" {
		t.Errorf("Entry status = %q, want %q", entry.Status, "superseded")
	}

	// Verify entry SupersededBy points to rejection proposal
	if entry.SupersededBy != "prop-456" {
		t.Errorf("SupersededBy = %q, want %q", entry.SupersededBy, "prop-456")
	}

	// Verify rejection decision has action=rejected
	if rejectionDecision.Action != "rejected" {
		t.Errorf("Rejection decision Action = %q, want %q", rejectionDecision.Action, "rejected")
	}

	// Verify rejection decision has the correct proposal ID
	if rejectionDecision.ProposalID != "prop-456" {
		t.Errorf("Rejection decision ProposalID = %q, want %q", rejectionDecision.ProposalID, "prop-456")
	}

	// Verify rejection decision reason is preserved
	if rejectionDecision.Reason != "better approach found" {
		t.Errorf("Rejection decision Reason = %q, want %q", rejectionDecision.Reason, "better approach found")
	}
}

func TestPromote_DoctrineRule(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-doctrine-001",
			Type:           "doctrine_rule",
			Title:          "Use consistent error handling",
			ProposedChange: "All errors must be logged and wrapped with context",
			Rationale:      "Helps with debugging and error tracking",
		},
		RunID:  "run-001",
		SpecID: "spec-001",
	}

	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir

	decision, err := Promote(
		pp,
		"", // no override
		"", // no override
		"", // no override
		docStore,
		nil,     // playbook not needed
		"local", // use local scope
		tmpDir,
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Verify decision created
	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify materialized ID has promoted- prefix
	if !strings.HasPrefix(decision.MaterializedID, "promoted-") {
		t.Errorf("MaterializedID %q should start with 'promoted-'", decision.MaterializedID)
	}

	// Verify rule was saved to doctrine store with correct ID and provenance
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]
	if rule.ID != decision.MaterializedID {
		t.Errorf("Rule ID = %q, want %q", rule.ID, decision.MaterializedID)
	}

	// Verify source provenance is set correctly
	expectedSource := "promoted:prop-doctrine-001"
	if rule.Source != expectedSource {
		t.Errorf("Source = %q, want %q", rule.Source, expectedSource)
	}

	if rule.Status != "active" {
		t.Errorf("Status = %q, want %q", rule.Status, "active")
	}
}

func TestPromote_DoctrineRule_ProvenanceFields(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-provenance-001",
			Type:           "doctrine_rule",
			Title:          "Always document public APIs",
			ProposedChange: "Every public function must have clear documentation explaining parameters and return values",
			Rationale:      "Improves API usability and reduces misuse",
		},
		RunID:  "run-provenance-001",
		SpecID: "spec-provenance-001",
	}

	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir

	decision, err := Promote(
		pp,
		"", // no title override
		"", // no change override
		"", // no rationale override
		docStore,
		nil,     // playbookStore not needed for doctrine_rule
		"local", // use local scope
		tmpDir,
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Load doctrine and verify rule was materialized with provenance fields
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// Verify provenance fields are set correctly
	if rule.SourceProposalID != "prop-provenance-001" {
		t.Errorf("SourceProposalID = %q, want %q", rule.SourceProposalID, "prop-provenance-001")
	}

	if rule.SourceRunID != "run-provenance-001" {
		t.Errorf("SourceRunID = %q, want %q", rule.SourceRunID, "run-provenance-001")
	}

	if rule.SourceSpecID != "spec-provenance-001" {
		t.Errorf("SourceSpecID = %q, want %q", rule.SourceSpecID, "spec-provenance-001")
	}
}

func TestPromote_PlaybookEntry(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-pb-001",
			Type:           "optimization_strategy",
			Title:          "Use connection pooling",
			ProposedChange: "Implement connection pool for database queries",
			Rationale:      "Reduces connection overhead",
		},
		RunID:  "run-pb-001",
		SpecID: "spec-pb-001",
	}

	pbStore := &playbook.Store{Dir: tmpDir}

	decision, err := Promote(
		pp,
		"",
		"",
		"",
		nil, // doctrine not needed
		pbStore,
		"local", // use local scope
		tmpDir,
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify materialized entry was saved to playbook store
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]
	if entry.ID != decision.MaterializedID {
		t.Errorf("Entry ID = %q, want %q", entry.ID, decision.MaterializedID)
	}

	if entry.Type != "optimization_strategy" {
		t.Errorf("Type = %q, want %q", entry.Type, "optimization_strategy")
	}

	if entry.Title != "Use connection pooling" {
		t.Errorf("Title = %q, want %q", entry.Title, "Use connection pooling")
	}

	if entry.Status != "active" {
		t.Errorf("Status = %q, want %q", entry.Status, "active")
	}
}

func TestPromote_FieldOverrides(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-override-001",
			Type:           "design_pattern",
			Title:          "Original Title",
			ProposedChange: "Original change description",
			Rationale:      "Original rationale",
		},
		RunID:  "run-override-001",
		SpecID: "spec-override-001",
	}

	pbStore := &playbook.Store{Dir: tmpDir}

	decision, err := Promote(
		pp,
		"Refined Title",       // override title
		"Refined change text", // override change
		"Refined rationale",   // override rationale
		nil,
		pbStore,
		"local", // use local scope
		tmpDir,
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Verify decision records overridden values
	if decision.ApprovedTitle != "Refined Title" {
		t.Errorf("ApprovedTitle = %q, want %q", decision.ApprovedTitle, "Refined Title")
	}

	if decision.ApprovedChange != "Refined change text" {
		t.Errorf("ApprovedChange = %q, want %q", decision.ApprovedChange, "Refined change text")
	}

	if decision.ApprovedRationale != "Refined rationale" {
		t.Errorf("ApprovedRationale = %q, want %q", decision.ApprovedRationale, "Refined rationale")
	}

	// Verify playbook entry uses overridden values
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]
	if entry.Title != "Refined Title" {
		t.Errorf("Entry Title = %q, want %q", entry.Title, "Refined Title")
	}

	if entry.Content != "Refined change text" {
		t.Errorf("Entry Content = %q, want %q", entry.Content, "Refined change text")
	}

	if entry.Rationale != "Refined rationale" {
		t.Errorf("Entry Rationale = %q, want %q", entry.Rationale, "Refined rationale")
	}
}

func TestPromote_Duplicate(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dup-test-001",
			Type:           "doctrine_rule",
			Title:          "Never share database connections",
			ProposedChange: "Database connections must be isolated per request",
			Rationale:      "Prevents data leaks",
		},
		RunID:  "run-dup-test-001",
		SpecID: "spec-dup-test-001",
	}

	// Compute the materialized ID
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(pp.Proposal.ProposedChange, " ")
	normalized = strings.TrimSpace(normalized)
	hashInput := fmt.Sprintf("%s:%s", pp.Proposal.Type, normalized)
	hash := sha256.Sum256([]byte(hashInput))
	hexStr := fmt.Sprintf("%x", hash)
	expectedMaterializedID := "promoted-" + hexStr[:8]

	// Pre-populate doctrine store with a rule having the same materialized ID
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir
	existingDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:      expectedMaterializedID,
				Summary: "Existing rule with this ID",
				Status:  "active",
			},
		},
	}
	if err := docStore.Save(existingDoctrine); err != nil {
		t.Fatalf("Failed to save doctrine: %v", err)
	}

	decision, err := Promote(
		pp,
		"",
		"",
		"",
		docStore,
		nil,
		"local", // use local scope
		"",      // evidenceDir
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Verify duplicate was detected and recorded
	if decision.DuplicateOf != expectedMaterializedID {
		t.Errorf("DuplicateOf = %q, want %q", decision.DuplicateOf, expectedMaterializedID)
	}

	if decision.MaterializedID != expectedMaterializedID {
		t.Errorf("MaterializedID = %q, want %q", decision.MaterializedID, expectedMaterializedID)
	}

	// Verify no new rule was materialized (store should still have only 1 rule)
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule (no duplicate materialized), got %d", len(loadedDoctrine.Rules))
	}

	// Verify original rule is unchanged
	rule := loadedDoctrine.Rules[0]
	if rule.Summary != "Existing rule with this ID" {
		t.Errorf("Rule was modified during duplicate check")
	}
}

func TestPromote_NilDoctrineStoreForDoctrineRule(t *testing.T) {
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-nil-doc-test",
			Type:           "doctrine_rule",
			Title:          "Test Rule",
			ProposedChange: "Test change text",
			Rationale:      "Test rationale",
		},
		RunID:  "run-nil-doc-test",
		SpecID: "spec-nil-doc-test",
	}

	// Call Promote with nil doctrineStore for a doctrine_rule proposal
	decision, err := Promote(
		pp,
		"",
		"",
		"",
		nil, // doctrineStore is nil
		nil,
		"local", // use local scope
		"",      // evidenceDir
	)

	if err == nil {
		t.Fatal("Promote should error when doctrineStore is nil for doctrine_rule proposal")
	}

	if decision != nil {
		t.Error("Promote should return nil decision on error")
	}

	// Verify the error message mentions the doctrineStore requirement
	if !strings.Contains(err.Error(), "doctrineStore is required") {
		t.Errorf("Error message = %q, should mention doctrineStore requirement", err.Error())
	}
}

func TestPromote_NilPlaybookStoreForPlaybookType(t *testing.T) {
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-nil-pb-test",
			Type:           "planner_heuristic",
			Title:          "Test Heuristic",
			ProposedChange: "Test change text",
			Rationale:      "Test rationale",
		},
		RunID:  "run-nil-pb-test",
		SpecID: "spec-nil-pb-test",
	}

	// Call Promote with nil playbookStore for a non-doctrine proposal type
	decision, err := Promote(
		pp,
		"",
		"",
		"",
		nil,
		nil,     // playbookStore is nil
		"local", // use local scope
		"",      // evidenceDir
	)

	if err == nil {
		t.Fatal("Promote should error when playbookStore is nil for non-doctrine proposal")
	}

	if decision != nil {
		t.Error("Promote should return nil decision on error")
	}

	// Verify the error message mentions the playbookStore requirement
	if !strings.Contains(err.Error(), "playbookStore is required") {
		t.Errorf("Error message = %q, should mention playbookStore requirement", err.Error())
	}
}

// TestAccept_DuplicateDetection_Doctrine_SupersededDoesNotBlock verifies that accepting a proposal
// whose ID matches a superseded doctrine rule does NOT treat it as a duplicate and materializes a new active rule.
// This tests AC11: duplicates are only detected for "active" entries, not "superseded" ones.
func TestAccept_DuplicateDetection_Doctrine_SupersededDoesNotBlock(t *testing.T) {
	tmpDir := t.TempDir()

	// Define a proposal with specific change text
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dup-superseded-001",
			Type:           "doctrine_rule",
			Title:          "Avoid global state",
			ProposedChange: "Never use global state in module scope",
			Rationale:      "Improves testability",
		},
		RunID:  "run-dup-superseded-001",
		SpecID: "spec-dup-superseded-001",
	}

	// Pre-compute the materialized ID
	normalized := regexp.MustCompile(`\s+`).ReplaceAllString(pp.Proposal.ProposedChange, " ")
	normalized = strings.TrimSpace(normalized)
	hashInput := fmt.Sprintf("%s:%s", pp.Proposal.Type, normalized)
	hash := sha256.Sum256([]byte(hashInput))
	hexStr := fmt.Sprintf("%x", hash)
	expectedMaterializedID := "promoted-" + hexStr[:8]

	// Create doctrine store with an existing SUPERSEDED rule with the same ID
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir
	existingDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{
			{
				ID:           expectedMaterializedID,
				Summary:      "Old Rule (Superseded)",
				Status:       "superseded",
				SupersededBy: "promoted-newer-id",
			},
		},
	}
	if err := docStore.Save(existingDoctrine); err != nil {
		t.Fatalf("Failed to save doctrine: %v", err)
	}

	// Call Promote - should NOT treat as duplicate since the existing rule is superseded
	decision, err := Promote(
		pp,
		"", // no title override
		"", // no change override
		"", // no rationale override
		docStore,
		nil,     // playbookStore not needed
		"local", // use local scope
		"",      // evidenceDir
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify action is accepted
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify MaterializedID is correct
	if decision.MaterializedID != expectedMaterializedID {
		t.Errorf("MaterializedID = %q, want %q", decision.MaterializedID, expectedMaterializedID)
	}

	// KEY: Verify DuplicateOf is EMPTY (not set) - this is the critical check for AC11
	// The existing rule is superseded, so it should NOT block the new promotion
	if decision.DuplicateOf != "" {
		t.Errorf("DuplicateOf = %q, want empty string (superseded rule should not block)", decision.DuplicateOf)
	}

	// Verify that a NEW rule WAS created (should now have 2 rules: the old superseded one and the new active one)
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 2 {
		t.Fatalf("Expected 2 rules (old superseded + new active), got %d", len(loadedDoctrine.Rules))
	}

	// Find the new active rule
	var newRule *doctrine.Rule
	var foundSuperseded bool
	for i := range loadedDoctrine.Rules {
		if loadedDoctrine.Rules[i].Status == "superseded" {
			foundSuperseded = true
		} else if loadedDoctrine.Rules[i].Status == "active" && loadedDoctrine.Rules[i].ID == expectedMaterializedID {
			newRule = &loadedDoctrine.Rules[i]
		}
	}

	if !foundSuperseded {
		t.Error("Expected to find the old superseded rule still in store")
	}

	if newRule == nil {
		t.Error("Expected to find new active rule with same ID as superseded rule")
	} else {
		if newRule.Status != "active" {
			t.Errorf("New rule status = %q, want %q", newRule.Status, "active")
		}
	}
}

// TestAccept_DuplicateDetection_Playbook_SupersededDoesNotBlock verifies that accepting a proposal
// whose ID matches a superseded playbook entry does NOT treat it as a duplicate and materializes a new active entry.
// This tests AC11 for playbook entries: duplicates are only detected for "active" entries, not "superseded" ones.
func TestAccept_DuplicateDetection_Playbook_SupersededDoesNotBlock(t *testing.T) {
	tmpDir := t.TempDir()

	// Define a proposal
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dup-pb-superseded-001",
			Type:           "planner_heuristic",
			Title:          "Prefer async operations",
			ProposedChange: "Use async/await for I/O operations to avoid blocking",
			Rationale:      "Improves responsiveness",
		},
		RunID:  "run-dup-pb-superseded-001",
		SpecID: "spec-dup-pb-superseded-001",
	}

	// Compute what the ID will be
	expectedMaterializedID := playbook.ComputeID(pp.Proposal.Type, pp.Proposal.ProposedChange)

	// Create playbook store with an existing SUPERSEDED entry with the same ID
	pbStore := &playbook.Store{Dir: tmpDir}
	existingEntries := []playbook.Entry{
		{
			ID:           expectedMaterializedID,
			Type:         pp.Proposal.Type,
			Title:        "Old Heuristic (Superseded)",
			Content:      pp.Proposal.ProposedChange,
			Status:       "superseded",
			SupersededBy: "pb-newer-id",
		},
	}
	if err := pbStore.Save(existingEntries); err != nil {
		t.Fatalf("Failed to save playbook entries: %v", err)
	}

	// Call Promote - should NOT treat as duplicate since the existing entry is superseded
	decision, err := Promote(
		pp,
		"",  // no title override
		"",  // no change override
		"",  // no rationale override
		nil, // doctrineStore not needed
		pbStore,
		"local", // use local scope
		"",      // evidenceDir
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Verify action is accepted
	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Verify MaterializedID is correct
	if decision.MaterializedID != expectedMaterializedID {
		t.Errorf("MaterializedID = %q, want %q", decision.MaterializedID, expectedMaterializedID)
	}

	// KEY: Verify DuplicateOf is EMPTY (not set) - this is the critical check for AC11
	// The existing entry is superseded, so it should NOT block the new promotion
	if decision.DuplicateOf != "" {
		t.Errorf("DuplicateOf = %q, want empty string (superseded entry should not block)", decision.DuplicateOf)
	}

	// Verify that a NEW entry WAS created (should now have 2 entries: the old superseded one and the new active one)
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 2 {
		t.Fatalf("Expected 2 entries (old superseded + new active), got %d", len(loadedEntries))
	}

	// Find the new active entry
	var newEntry *playbook.Entry
	var foundSuperseded bool
	for i := range loadedEntries {
		if loadedEntries[i].Status == "superseded" {
			foundSuperseded = true
		} else if loadedEntries[i].Status == "active" && loadedEntries[i].ID == expectedMaterializedID {
			newEntry = &loadedEntries[i]
		}
	}

	if !foundSuperseded {
		t.Error("Expected to find the old superseded entry still in store")
	}

	if newEntry == nil {
		t.Error("Expected to find new active entry with same ID as superseded entry")
	} else {
		if newEntry.Status != "active" {
			t.Errorf("New entry status = %q, want %q", newEntry.Status, "active")
		}
	}
}

// TestPromote_DoctrineRule_WithGlobalScope verifies that when PendingProposal.Scope is 'global',
// the materialized doctrine Rule has Scope='global'.
func TestPromote_DoctrineRule_WithGlobalScope(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-global-scope-001",
			Type:           "doctrine_rule",
			Title:          "Use global configuration approach",
			ProposedChange: "All config should be globally accessible via a singleton pattern",
			Rationale:      "Simplifies dependency passing",
		},
		RunID:  "run-global-scope-001",
		SpecID: "spec-global-scope-001",
		Scope:  "global",
	}

	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir

	decision, err := Promote(
		pp,
		"", // no override
		"", // no override
		"", // no override
		docStore,
		nil,      // playbook not needed
		"global", // use global scope
		tmpDir,
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Load doctrine store and verify rule has global scope
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// KEY: Verify that Scope is 'global', not the default '*'
	if rule.Scope != "global" {
		t.Errorf("Rule Scope = %q, want %q", rule.Scope, "global")
	}
}

// TestPromote_PlaybookEntry_WithGlobalScope verifies that when PendingProposal.Scope is 'global',
// the materialized playbook Entry has Scope='global'.
func TestPromote_PlaybookEntry_WithGlobalScope(t *testing.T) {
	tmpDir := t.TempDir()

	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-global-pb-001",
			Type:           "backend_pattern",
			Title:          "Use dependency injection globally",
			ProposedChange: "All backend services should use dependency injection for loose coupling",
			Rationale:      "Improves testability and maintainability across all services",
		},
		RunID:  "run-global-pb-001",
		SpecID: "spec-global-pb-001",
		Scope:  "global",
	}

	pbStore := &playbook.Store{Dir: tmpDir}

	decision, err := Promote(
		pp,
		"",  // no override
		"",  // no override
		"",  // no override
		nil, // doctrineStore not needed
		pbStore,
		"global", // use global scope
		tmpDir,
	)

	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}

	// Load playbook store and verify entry has global scope
	loadedEntries, err := pbStore.Load()
	if err != nil {
		t.Fatalf("Failed to load playbook entries: %v", err)
	}

	if len(loadedEntries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(loadedEntries))
	}

	entry := loadedEntries[0]

	// KEY: Verify that Scope is 'global'
	if entry.Scope != "global" {
		t.Errorf("Entry Scope = %q, want %q", entry.Scope, "global")
	}
}

func TestPromote_DoctrineRule_DefaultScope(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a pending proposal with type "doctrine_rule" and empty Scope
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-default-scope",
			Type:           "doctrine_rule",
			Title:          "Use clear naming conventions",
			ProposedChange: "Enforce descriptive variable names across the codebase",
			Rationale:      "Improves code readability and maintainability",
		},
		RunID:  "run-default-scope",
		SpecID: "spec-default-scope",
		Scope:  "", // Empty scope - should default to "*"
	}

	// Create doctrine store
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir

	// Call Promote with empty scope parameter for backward compatibility
	decision, err := Promote(
		pp,
		"", // no title override
		"", // no change override
		"", // no rationale override
		docStore,
		nil, // playbookStore not needed for doctrine_rule
		"",  // empty scope parameter - should default to "*"
		"",  // evidenceDir
	)

	// Verify decision was created successfully
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	// Load doctrine and verify rule was materialized
	loadedDoctrine, err := docStore.Load()
	if err != nil {
		t.Fatalf("Failed to load doctrine: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// KEY: Verify that Scope defaults to "*" when empty (backward compatible)
	if rule.Scope != "*" {
		t.Errorf("Rule Scope = %q, want %q", rule.Scope, "*")
	}
}

func TestDismissedProposalCannotBeRedecided(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a proposal
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dismissed-001",
			Type:           "doctrine_rule",
			Title:          "Test Proposal",
			ProposedChange: "Some test change",
		},
		RunID:  "run-001",
		SpecID: "spec-001",
	}

	// Create an evidence directory with a dismissed decision for this proposal
	dismissedDecision := Decision{
		ProposalID:  "prop-dismissed-001",
		Action:      "dismissed",
		DismissedBy: "prop-accepted-001",
		DecidedAt:   time.Now(),
	}

	// Save the dismissed decision to the evidence directory
	err := SaveDecision(tmpDir, dismissedDecision)
	if err != nil {
		t.Fatalf("Failed to save dismissed decision: %v", err)
	}

	// Load decisions and validate that the proposal cannot be re-decided
	decisions, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load decisions: %v", err)
	}
	err = ValidateTerminalState(pp.Proposal.ID, decisions)

	// Verify that ValidateTerminalState returns an error
	if err == nil {
		t.Fatal("ValidateTerminalState should return an error for dismissed proposals")
	}

	// Verify the error message mentions dismissed
	errMsg := err.Error()
	if !strings.Contains(errMsg, "dismissed") {
		t.Errorf("Error message should mention 'dismissed', got: %q", errMsg)
	}

	if !strings.Contains(errMsg, "cannot be re-decided") {
		t.Errorf("Error message should mention 'cannot be re-decided', got: %q", errMsg)
	}
}

func TestValidateTerminalState_NoDecisions(t *testing.T) {
	tmpDir := t.TempDir()

	// Load decisions (empty since nothing was saved)
	decisions, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load decisions: %v", err)
	}

	// Validate a proposal with no existing decisions
	err = ValidateTerminalState("prop-no-decision", decisions)

	// Should not return an error since there are no decisions
	if err != nil {
		t.Fatalf("ValidateTerminalState should not error when no decisions exist, got: %v", err)
	}
}

func TestValidateTerminalState_RejectedProposalCanBeAccepted(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an evidence directory with a rejected decision for a proposal
	rejectedDecision := Decision{
		ProposalID: "prop-rejected-001",
		Action:     "rejected",
		Reason:     "Not ready yet",
		DecidedAt:  time.Now(),
	}

	// Save the rejected decision
	err := SaveDecision(tmpDir, rejectedDecision)
	if err != nil {
		t.Fatalf("Failed to save rejected decision: %v", err)
	}

	// Load decisions and validate that the rejected proposal CAN be re-decided (rejected is not terminal)
	decisions, err := LoadDecisions(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load decisions: %v", err)
	}
	err = ValidateTerminalState("prop-rejected-001", decisions)

	// Should not return an error since rejected is not a terminal state
	if err != nil {
		t.Fatalf("ValidateTerminalState should not error for rejected proposals, got: %v", err)
	}
}

func TestPromote_DismissedProposal_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create stores
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir

	pbStore := &playbook.Store{Dir: tmpDir}

	// Create a dismissed decision for a proposal in the evidence directory
	dismissedDecision := Decision{
		ProposalID:  "prop-dismissed-123",
		Action:      "dismissed",
		DismissedBy: "prop-accepted-999",
		DecidedAt:   time.Now(),
	}

	// Save the dismissed decision to the evidence directory
	err := SaveDecision(tmpDir, dismissedDecision)
	if err != nil {
		t.Fatalf("Failed to save dismissed decision: %v", err)
	}

	// Create a pending proposal with the same ID as the dismissed decision
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             "prop-dismissed-123",
			Type:           "doctrine_rule",
			Title:          "Test Proposal",
			ProposedChange: "Some test change",
		},
		RunID:  "run-001",
		SpecID: "spec-001",
	}

	// Call Promote - should fail because the proposal has a dismissed decision
	decision, err := Promote(
		pp,
		"",
		"",
		"",
		docStore,
		pbStore,
		"",
		tmpDir, // evidenceDir
	)

	// Verify that Promote returns an error
	if err == nil {
		t.Fatal("Promote should return an error for dismissed proposals")
	}

	// Verify that decision is nil
	if decision != nil {
		t.Errorf("Promote should return nil decision for dismissed proposals, got: %v", decision)
	}

	// Verify the error message mentions dismissed
	errMsg := err.Error()
	if !strings.Contains(errMsg, "dismissed") {
		t.Errorf("Error message should mention 'dismissed', got: %q", errMsg)
	}

	if !strings.Contains(errMsg, "cannot be re-decided") {
		t.Errorf("Error message should mention 'cannot be re-decided', got: %q", errMsg)
	}
}

func TestPromote_AcceptedProposal_AllowsRePromotion(t *testing.T) {
	tmpDir := t.TempDir()

	proposalID := "prop-already-accepted-123"

	// Pre-populate evidence dir with an accepted decision for this proposal
	existingDecision := Decision{
		ProposalID: proposalID,
		Action:     "accepted",
		DecidedAt:  time.Now(),
	}
	if err := SaveDecision(tmpDir, existingDecision); err != nil {
		t.Fatalf("Failed to save existing decision: %v", err)
	}

	// Create initial stores
	docStore := doctrine.NewFSStore()
	docStore.Dir = tmpDir
	initialDoctrine := doctrine.Doctrine{
		Rules: []doctrine.Rule{},
	}
	if err := docStore.Save(initialDoctrine); err != nil {
		t.Fatalf("Failed to save initial doctrine: %v", err)
	}

	pbStore := &playbook.Store{Dir: tmpDir}
	if err := pbStore.Save([]playbook.Entry{}); err != nil {
		t.Fatalf("Failed to save initial playbook entries: %v", err)
	}

	// Create a proposal with the same ID that already has an accepted decision
	pp := &PendingProposal{
		Proposal: &reviewdistiller.Proposal{
			ID:             proposalID,
			Title:          "Test Proposal",
			ProposedChange: "implement feature",
			Type:           "playbook_rule",
		},
		RunID:  "run-123",
		SpecID: "spec-456",
	}

	// Call Promote - should succeed because accepted is not terminal
	decision, err := Promote(
		pp,
		"",
		"",
		"",
		docStore,
		pbStore,
		"",
		tmpDir, // evidenceDir
	)

	if err != nil {
		t.Fatalf("Promote failed with accepted existing decision: %v", err)
	}

	if decision == nil {
		t.Fatal("Promote returned nil decision")
	}

	if decision.ProposalID != proposalID {
		t.Errorf("ProposalID = %q, want %q", decision.ProposalID, proposalID)
	}

	if decision.Action != "accepted" {
		t.Errorf("Action = %q, want %q", decision.Action, "accepted")
	}
}
