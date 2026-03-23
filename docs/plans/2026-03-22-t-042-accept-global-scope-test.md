# T-042: Accept Global Scope Doctrine Rule Test

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a CLI scenario test that accepts a doctrine_rule proposal with `--scope global`, materializing the rule under `{storeDir}/global/doctrine` with `Scope='global'`.

**Architecture:** Create a test file `review_proposals_scenario_accept_global_test.go` that follows the existing pattern from `review_proposals_scenario_accept_cli_test.go` and `review_proposals_scenario_accept_global_test.go`. The test will seed a doctrine_rule proposal, invoke the accept CLI command with `--scope global`, and verify the rule is materialized in the global doctrine store with the correct Scope field.

**Tech Stack:** Go testing, cobra CLI framework, JSON marshaling

---

### Task 1: Create failing test file

**Files:**
- Create: `cmd/gromit-next/review_proposals_scenario_accept_global_test.go`
- Reference: `cmd/gromit-next/review_proposals_scenario_accept_cli_test.go` (line 18-93, 121-174) for structure pattern
- Reference: `cmd/gromit-next/review_proposals_scenario_accept_global_test.go` (line 78-79) for global scope paths

**Step 1: Write the failing test**

Create the test file with the following complete code:

```go
package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/proposaltriage"
	"github.com/danabrams/gromit/internal/next/reviewdistiller"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_AcceptCLIWithGlobalScope(t *testing.T) {
	// === Seed ===
	tmp := t.TempDir()
	store := runstore.NewStore(tmp)
	projectID := "test-project"
	runID := "run-003"

	// Create run
	run := &runstore.RunState{
		RunID:     runID,
		ProjectID: projectID,
		SpecID:    "spec-test-global",
		Status:    "completed",
		StartedAt: time.Date(2026, 3, 21, 12, 0, 0, 0, time.UTC),
	}
	run.NormalizeNilFields()
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Create project directories (needed for discovery)
	projectDir := filepath.Join(tmp, "projects", projectID)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	// Create evidence directory with proposals
	evidenceDir := store.RunEvidenceDir(runID)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	proposalID := "run-003-proposal-global-001"
	distResult := reviewdistiller.DistillationResult{
		RunID:     runID,
		SpecID:    "spec-test-global",
		Outcome:   "accepted",
		ModelTier: reviewdistiller.TierMedium,
		CreatedAt: time.Date(2026, 3, 21, 12, 20, 0, 0, time.UTC),
		Proposals: []reviewdistiller.Proposal{
			{
				ID:                  proposalID,
				Type:                "doctrine_rule",
				Title:               "Global Convention: Always validate input at system boundaries",
				WhatHappened:        "Invalid input reached internal handlers",
				WhatWasMissing:      "Centralized input validation at API boundaries",
				ProposedChange:      "Validate all external input at system entry points before passing to domain logic",
				Rationale:           "Prevents invalid state and security issues downstream",
				Confidence:          "high",
				ConfidenceRationale: "Fundamental security best practice",
				EvidenceReferences:  []string{},
			},
		},
	}

	proposalsJSON, err := json.MarshalIndent(distResult, "", "  ")
	if err != nil {
		t.Fatalf("marshal proposals: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evidenceDir, "distillation-proposals.json"), proposalsJSON, 0o644); err != nil {
		t.Fatalf("write proposals: %v", err)
	}

	// === Invoke ===
	// Create the accept command
	acceptCmd := newReviewProposalsAcceptCmd()

	// Set up command with --scope global flag
	acceptCmd.SetArgs([]string{
		proposalID,
		"--store-dir", tmp,
		"--scope", "global",
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Execute command
	err = acceptCmd.Execute()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("accept command with --scope global failed: %v", err)
	}

	// Read output
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	outputStr := string(output)

	// === Assert ===

	// 1. Command output contains expected messages
	if !strings.Contains(outputStr, "Proposal") || !strings.Contains(outputStr, "accepted") {
		t.Errorf("expected 'accepted' in output, got: %s", outputStr)
	}

	if !strings.Contains(outputStr, "Materialized ID:") {
		t.Errorf("expected 'Materialized ID:' in output, got: %s", outputStr)
	}

	// 2. Decision file was created in run evidence directory
	runEvidenceDir := filepath.Join(tmp, "runs", runID, "evidence")
	decisionsPath := filepath.Join(runEvidenceDir, "proposal-decisions.json")
	decisionsRaw, err := os.ReadFile(decisionsPath)
	if err != nil {
		t.Fatalf("read decisions file: %v", err)
	}

	var savedDecisions []proposaltriage.Decision
	if err := json.Unmarshal(decisionsRaw, &savedDecisions); err != nil {
		t.Fatalf("unmarshal decisions: %v", err)
	}

	if len(savedDecisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(savedDecisions))
	}

	decision := savedDecisions[0]

	// 3. Decision has correct properties
	if decision.ProposalID != proposalID {
		t.Errorf("decision proposal_id = %q, want %q", decision.ProposalID, proposalID)
	}

	if decision.Action != "accepted" {
		t.Errorf("decision action = %q, want 'accepted'", decision.Action)
	}

	// 4. **KEY ASSERTION**: Doctrine rule was created in GLOBAL store (not project store)
	globalDoctrineDir := filepath.Join(tmp, "global", "doctrine")
	doctrineStore := doctrine.NewFSStore()
	doctrineStore.Dir = globalDoctrineDir
	loadedDoctrine, err := doctrineStore.Load()
	if err != nil {
		t.Fatalf("load doctrine from global dir: %v", err)
	}

	if len(loadedDoctrine.Rules) != 1 {
		t.Fatalf("expected 1 doctrine rule in global store, got %d", len(loadedDoctrine.Rules))
	}

	rule := loadedDoctrine.Rules[0]

	// 5. Rule was NOT created in project store
	projectDoctrineDir := filepath.Join(projectDir, "doctrine")
	projectDoctrineStore := doctrine.NewFSStore()
	projectDoctrineStore.Dir = projectDoctrineDir
	projectDoctrine, err := projectDoctrineStore.Load()
	if err == nil && len(projectDoctrine.Rules) > 0 {
		t.Errorf("rule should not be in project store, but found %d rules", len(projectDoctrine.Rules))
	}

	// 6. **KEY ASSERTION**: Rule Scope field is "global"
	if rule.Scope != "global" {
		t.Errorf("rule scope = %q, want 'global'", rule.Scope)
	}

	// 7. Rule has other expected properties
	if rule.Summary != "Global Convention: Always validate input at system boundaries" {
		t.Errorf("rule summary = %q, want proposal title", rule.Summary)
	}

	expectedSource := "promoted:" + proposalID
	if rule.Source != expectedSource {
		t.Errorf("rule source = %q, want %q", rule.Source, expectedSource)
	}

	if rule.Status != "active" {
		t.Errorf("rule status = %q, want 'active'", rule.Status)
	}

	// 8. Materialized ID in decision matches the created rule
	if decision.MaterializedID != rule.ID {
		t.Errorf("materialized_id = %q, want %q (matching rule ID)", decision.MaterializedID, rule.ID)
	}

	// 9. Proposal no longer appears in pending list
	pendingAfter, err := proposaltriage.DiscoverPending(tmp, projectID, nil, nil)
	if err != nil {
		t.Fatalf("DiscoverPending after accept: %v", err)
	}

	if len(pendingAfter) != 0 {
		t.Fatalf("expected 0 pending proposals after accept, got %d", len(pendingAfter))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dabrams/gromit && go test -run TestScenario_AcceptCLIWithGlobalScope -v ./cmd/gromit-next/`

Expected: FAIL with error indicating the test function exists but the code doesn't yet properly handle `--scope global` flag in the accept command.

**Step 3: Verify test file is in correct location**

Run: `cd /Users/dabrams/gromit && ls -la cmd/gromit-next/review_proposals_scenario_accept_global_test.go`

Expected: File exists in the cmd/gromit-next directory.

**Step 4: Commit test file**

```bash
cd /Users/dabrams/gromit
git add cmd/gromit-next/review_proposals_scenario_accept_global_test.go
git commit -m "red: add scenario test for accepting doctrine rule with --scope global

Test seeds a doctrine_rule proposal and invokes accept CLI with --scope global flag.
Asserts that:
- Rule is materialized in {storeDir}/global/doctrine (not project store)
- Rule.Scope field is set to 'global'
- Decision is properly recorded in evidence directory
- Proposal is removed from pending list"
```

---

## Execution Notes

- The test follows existing patterns from both `review_proposals_scenario_accept_cli_test.go` (CLI invocation) and `review_proposals_scenario_accept_global_test.go` (global scope handling)
- Key assertions verify both the file location (`global/doctrine` not `projects/{projectID}/doctrine`) and the Rule.Scope field value
- The test is designed to verify end-to-end behavior with the `--scope global` CLI flag
