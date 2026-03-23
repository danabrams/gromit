package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/next/reviewpacket"
	"github.com/danabrams/gromit/internal/next/runstore"
)

func TestScenario_MalformedReviewJSON_DoesNotFailAccept(t *testing.T) {
	// ── Seed ──────────────────────────────────────────────────────────────
	tmp := t.TempDir()
	storeDir := filepath.Join(tmp, "store")
	specsDir := filepath.Join(tmp, "docs", "specs")

	store := runstore.NewStore(storeDir)

	run := &runstore.RunState{
		RunID:     "run-malformed-review",
		SpecID:    "spec-0042",
		ProjectID: "test-project",
		Status:    runstore.StatusCompleted,
		StartedAt: time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
		EndedAt:   time.Date(2026, 3, 23, 10, 30, 0, 0, time.UTC),
	}
	if err := store.Save(run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	evidenceDir := store.RunEvidenceDir("run-malformed-review")
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}

	// Write malformed review.json
	if err := os.WriteFile(filepath.Join(evidenceDir, "review.json"), []byte(`{bad json`), 0o644); err != nil {
		t.Fatalf("write review.json: %v", err)
	}

	// Create required review packet files so reviewRecord succeeds
	productReview := reviewpacket.ProductReview{
		RunID:         "run-malformed-review",
		SpecTitle:     "spec-0042",
		TerminalState: "completed",
		Summary:       "All good",
		BehaviorCards: []reviewpacket.BehaviorCard{},
		Surprises:     []string{},
	}
	processReview := reviewpacket.ProcessReview{
		TrustLevel:         "high",
		AutomaticProof:     "pass",
		MachineReview:      "pass",
		Acceptance:         "pass",
		DegradedFlags:      []string{},
		RecommendedPosture: "accept",
	}
	manualChecklist := reviewpacket.ManualChecklist{
		Items: []reviewpacket.ManualCheckItem{},
	}

	for name, v := range map[string]interface{}{
		"product-review.json":   productReview,
		"process-review.json":   processReview,
		"manual-checklist.json": manualChecklist,
	} {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(evidenceDir, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// ── Invoke ────────────────────────────────────────────────────────────
	cmd := newReviewRecordCmd()
	cmd.SetArgs([]string{
		"run-malformed-review",
		"--outcome", "accepted",
		"--summary", "Done",
		"--store-dir", storeDir,
		"--specs-dir", specsDir,
	})

	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	err := cmd.Execute()

	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdoutBytes, _ := io.ReadAll(rOut)
	stdoutStr := string(stdoutBytes)
	stderrBytes, _ := io.ReadAll(rErr)
	stderrStr := string(stderrBytes)

	// ── Assert ────────────────────────────────────────────────────────────

	// Accept succeeds (no error returned)
	if err != nil {
		t.Fatalf("expected command to succeed, got error: %v (stdout: %s, stderr: %s)", err, stdoutStr, stderrStr)
	}

	// review-outcome.json is written
	outcomePath := filepath.Join(evidenceDir, "review-outcome.json")
	if _, err := os.Stat(outcomePath); err != nil {
		t.Errorf("review-outcome.json not found: %v", err)
	}

	// stderr warns about unparseable review.json
	if !strings.Contains(stderrStr, "warning") {
		t.Errorf("expected warning in stderr, got:\n%s", stderrStr)
	}
	if !strings.Contains(stderrStr, "remediation spec") || !strings.Contains(stderrStr, "review.json") || !strings.Contains(stderrStr, "parse") {
		// The warning format is: "warning: failed to generate remediation spec: <error mentioning review.json or parse>"
		if !strings.Contains(stderrStr, "failed to generate remediation spec") {
			t.Errorf("expected stderr to warn about failed remediation spec generation, got:\n%s", stderrStr)
		}
	}

	// No remediation spec is generated
	expectedSpecPath := filepath.Join(specsDir, "spec-0042-remediation.md")
	if _, err := os.Stat(expectedSpecPath); err == nil {
		t.Error("remediation spec file should not be created when review.json is malformed")
	}

	// stdout should NOT contain a spec path
	if strings.Contains(stdoutStr, "remediation.md") {
		t.Errorf("stdout should not contain remediation spec path, got: %s", stdoutStr)
	}
}
