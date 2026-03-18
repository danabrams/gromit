package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/acceptor"
	"github.com/danabrams/gromit/internal/next/review"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/validator"
)

func TestAssembleBundle_CreatesEvidenceDir(t *testing.T) {
	dir := t.TempDir()
	evidenceDir := dir + "/evidence"
	b := NewBundler(evidenceDir)
	err := b.Init()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(evidenceDir); os.IsNotExist(err) {
		t.Fatal("evidence dir should exist")
	}
}

func TestBundler_WriteTaskResults(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	tasks := []runstore.Task{
		{TaskID: "t-001", Status: "done", Attempts: 1},
		{TaskID: "t-002", Status: "done", Attempts: 2},
	}
	err := b.WriteTaskResults(tasks)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "task-results.json"))
	if !strings.Contains(string(data), "t-001") {
		t.Fatal("task-results.json should contain task IDs")
	}
}

func TestBundler_WriteValidation(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	result := validator.FinalResult{Pass: true}
	err := b.WriteValidation(result)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "validation.json"))
	if !strings.Contains(string(data), `"pass":`) {
		t.Fatal("validation.json should contain pass status")
	}
}

func TestBundler_WriteMetrics(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	m := Metrics{
		TotalTokens:  5000,
		TotalCostUSD: 1.23,
		TotalTasks:   3,
		PassedTasks:  2,
		FailedTasks:  1,
		DurationMs:   45000,
		Cycles:       1,
		Invocations: []InvocationRecord{
			{Phase: "plan", Tier: "high", Model: "opus", TokensIn: 2000, TokensOut: 1000, DurationMs: 15000, Success: true},
		},
	}
	err := b.WriteMetrics(m)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "metrics.json"))
	if !strings.Contains(string(data), "5000") {
		t.Fatal("metrics.json should contain token count")
	}
}

func TestBundler_WriteDiffSummary(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	err := b.WriteDiffSummary("3 files changed, 120 insertions, 5 deletions")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "diff-summary.md"))
	if !strings.Contains(string(data), "120 insertions") {
		t.Fatal("diff-summary should contain stats")
	}
}

func TestBundler_WriteSummary(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	err := b.WriteSummary(SummaryInput{
		SpecID: "spec-001", Status: "ready_for_review", TaskCount: 3, PassCount: 3, Cycles: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	content := string(data)
	if !strings.Contains(content, "spec-001") {
		t.Fatal("summary should contain spec ID")
	}
	if !strings.Contains(content, "ready_for_review") {
		t.Fatal("summary should contain terminal status")
	}
}

func TestBundler_WriteReview(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	err := b.WriteReview(ReviewInput{
		TerminalState:     "ready_for_review",
		WhatChanged:       "Implemented parser package with 3 files",
		CycleHistory:      []CycleRecord{{Cycle: 1, TaskCount: 3, PassCount: 3}},
		ValidationResults: "All 3 checks passed",
		KnownRisks:        []string{"No error handling for malformed input"},
		RecommendedAction: "Merge after manual review of edge cases",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "review.md"))
	content := string(data)
	if !strings.Contains(content, "ready_for_review") {
		t.Fatal("review.md should contain terminal state")
	}
	if !strings.Contains(content, "Recommended Action") {
		t.Fatal("review.md should contain recommended action section")
	}
	if !strings.Contains(content, "Known Risks") {
		t.Fatal("review.md should contain known risks section")
	}
}

func TestBundler_WriteReview_BlockerSectionPresent(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	err := b.WriteReview(ReviewInput{
		TerminalState:     "needs_human",
		BlockerSummary:    "Circular dependency between parser and lexer packages",
		WhatChanged:       "Partial implementation",
		CycleHistory:      []CycleRecord{{Cycle: 1, TaskCount: 3, PassCount: 1}},
		ValidationResults: "1 of 3 checks passed",
		KnownRisks:        []string{"Incomplete"},
		RecommendedAction: "Human must resolve circular dependency",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "review.md"))
	content := string(data)
	if !strings.Contains(content, "## Blocker") {
		t.Fatal("review.md should contain ## Blocker section when BlockerSummary is set")
	}
	if !strings.Contains(content, "Circular dependency between parser and lexer packages") {
		t.Fatal("review.md should contain the blocker summary text")
	}
}

func TestBundler_WriteReview_BlockerSectionOmitted(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	err := b.WriteReview(ReviewInput{
		TerminalState:     "ready_for_review",
		WhatChanged:       "Implemented parser package",
		CycleHistory:      []CycleRecord{{Cycle: 1, TaskCount: 3, PassCount: 3}},
		ValidationResults: "All checks passed",
		KnownRisks:        []string{},
		RecommendedAction: "Merge",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "review.md"))
	content := string(data)
	if strings.Contains(content, "## Blocker") {
		t.Fatal("review.md should not contain ## Blocker section when BlockerSummary is empty")
	}
}

func TestBundler_WriteMetrics_NilInvocationsSerializesAsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	b.Init()
	m := Metrics{
		TotalTokens:  100,
		TotalCostUSD: 0.01,
		TotalTasks:   1,
		PassedTasks:  1,
		DurationMs:   1000,
		Cycles:       1,
		// Invocations intentionally left nil
	}
	err := b.WriteMetrics(m)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "metrics.json"))
	content := string(data)
	if strings.Contains(content, `"invocations": null`) {
		t.Fatal("invocations should serialize as [] not null")
	}
	if !strings.Contains(content, `"invocations": []`) {
		t.Fatal("invocations should serialize as empty array")
	}
}

func TestReviewInput_NormalizeNilFields(t *testing.T) {
	r := ReviewInput{}
	r.NormalizeNilFields()
	if r.CycleHistory == nil {
		t.Fatal("CycleHistory should not be nil after NormalizeNilFields")
	}
	if r.KnownRisks == nil {
		t.Fatal("KnownRisks should not be nil after NormalizeNilFields")
	}
}

func TestMetrics_NormalizeNilFields(t *testing.T) {
	m := Metrics{}
	m.NormalizeNilFields()
	if m.Invocations == nil {
		t.Fatal("Invocations should not be nil after NormalizeNilFields")
	}
}

func TestBundler_WriteReviewFindings(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	findings := map[string][]review.Finding{
		"spec_alignment": {
			{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Line: 42, Description: "missing validation"},
		},
		"code_quality": {
			{Facet: "code_quality", Severity: review.SeverityWarning, File: "router.go", Line: 10, Description: "long function"},
		},
	}

	output := ReviewFindingsOutput{
		Findings:        findings,
		DiffUnavailable: false,
	}

	if err := b.WriteReviewFindings(output); err != nil {
		t.Fatalf("WriteReviewFindings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("review.json should be a valid JSON object: %v", err)
	}

	specAlignmentRaw, ok := parsed["spec_alignment"]
	if !ok {
		t.Fatal("spec_alignment should be present")
	}
	if specAlignmentList, ok := specAlignmentRaw.([]interface{}); !ok || len(specAlignmentList) != 1 {
		t.Errorf("expected 1 spec_alignment finding, got %v", specAlignmentRaw)
	}

	codeQualityRaw, ok := parsed["code_quality"]
	if !ok {
		t.Fatal("code_quality should be present")
	}
	if codeQualityList, ok := codeQualityRaw.([]interface{}); !ok || len(codeQualityList) != 1 {
		t.Errorf("expected 1 code_quality finding, got %v", codeQualityRaw)
	}
}

func TestBundler_WriteAcceptanceResults(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	result := acceptor.AcceptanceResult{
		Results: []acceptor.CriterionResult{
			{Criterion: "multi-currency", Status: "fail", Rationale: "implement missing behavior"},
			{Criterion: "audit log", Status: "unclear", Rationale: "add tests or evidence to prove/disprove"},
		},
		AllPass:          false,
		HasFailOrUnclear: true,
	}

	if err := b.WriteAcceptanceResults(result); err != nil {
		t.Fatalf("WriteAcceptanceResults: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("acceptance.json should be a valid JSON object: %v", err)
	}
	results, ok := parsed["results"].([]interface{})
	if !ok || len(results) != 2 {
		t.Errorf("expected 2 results in results array, got %v", parsed["results"])
	}
	if parsed["all_pass"] != false {
		t.Error("all_pass should be false")
	}
	if parsed["has_fail_or_unclear"] != true {
		t.Error("has_fail_or_unclear should be true")
	}
}

func TestBundler_WriteReview_IncludesReviewFindings(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	input := ReviewInput{
		TerminalState:     "ready_for_review",
		WhatChanged:       "Added refund handler",
		CycleHistory:      []CycleRecord{{Cycle: 1, TaskCount: 4, PassCount: 4}},
		ValidationResults: "6/6 passed",
		KnownRisks:        []string{},
		RecommendedAction: "approve",
		ReviewFindings: []ReviewFindingSummary{
			{Facet: "spec_alignment", Count: 0, Severities: "none"},
			{Facet: "code_quality", Count: 1, Severities: "1 info"},
		},
		AcceptanceCriteria: []AcceptanceCriterionSummary{
			{Criterion: "returns 200", Status: "pass", Rationale: "test proves it"},
			{Criterion: "handles errors", Status: "pass", Rationale: "error tests exist"},
		},
	}

	if err := b.WriteReview(input); err != nil {
		t.Fatalf("WriteReview: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.md"))
	if err != nil {
		t.Fatalf("read review.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Review Findings") {
		t.Error("review.md should contain Review Findings section")
	}
	if !strings.Contains(content, "spec_alignment") {
		t.Error("review.md should list spec_alignment facet")
	}
	if !strings.Contains(content, "Acceptance Criteria") {
		t.Error("review.md should contain Acceptance Criteria section")
	}
	if !strings.Contains(content, "returns 200") {
		t.Error("review.md should list acceptance criteria")
	}
}

func TestBundler_WriteAcceptanceResults_AllPass(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	result := acceptor.AcceptanceResult{
		Results: []acceptor.CriterionResult{
			{Criterion: "returns 200", Status: "pass", Rationale: "integration test proves it"},
			{Criterion: "handles errors", Status: "pass", Rationale: "error handling tests exist"},
			{Criterion: "audit log", Status: "pass", Rationale: "log output verified"},
		},
		AllPass:          true,
		HasFailOrUnclear: false,
	}

	if err := b.WriteAcceptanceResults(result); err != nil {
		t.Fatalf("WriteAcceptanceResults: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("acceptance.json should be valid JSON: %v", err)
	}
	results, ok := parsed["results"].([]interface{})
	if !ok || len(results) != 3 {
		t.Errorf("expected 3 results in results array, got %v", parsed["results"])
	}
	if parsed["all_pass"] != true {
		t.Error("all_pass should be true")
	}
	if parsed["has_fail_or_unclear"] != false {
		t.Error("has_fail_or_unclear should be false")
	}
}

func TestBundler_WriteReviewFindings_EmptyFindings(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	// Pass an empty (non-nil) findings map
	findings := map[string][]review.Finding{}
	output := ReviewFindingsOutput{
		Findings:        findings,
		DiffUnavailable: false,
	}

	if err := b.WriteReviewFindings(output); err != nil {
		t.Fatalf("WriteReviewFindings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	// Should be valid JSON, not nil/null
	content := strings.TrimSpace(string(data))
	if content == "null" {
		t.Fatal("empty findings should serialize as valid JSON object, not null")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("review.json should be valid JSON: %v", err)
	}

	// Should have at least diff_unavailable field
	if _, ok := parsed["diff_unavailable"]; !ok {
		t.Fatal("diff_unavailable field should be present")
	}

	// Count facet entries (anything that's not diff_unavailable)
	facetCount := 0
	for k := range parsed {
		if k != "diff_unavailable" {
			facetCount++
		}
	}
	if facetCount != 0 {
		t.Errorf("expected 0 facet entries, got %d", facetCount)
	}
}

func TestBundler_WriteReviewFindings_WithDiffUnavailable(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	findings := map[string][]review.Finding{
		"spec_alignment": {
			{Facet: "spec_alignment", Severity: review.SeverityError, File: "handler.go", Line: 42, Description: "missing validation"},
		},
	}

	output := ReviewFindingsOutput{
		Findings:       findings,
		DiffUnavailable: true,
	}

	if err := b.WriteReviewFindings(output); err != nil {
		t.Fatalf("WriteReviewFindings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("review.json should be valid JSON: %v", err)
	}

	// Check that diff_unavailable field is present and true
	if diffUnavailable, ok := parsed["diff_unavailable"]; !ok {
		t.Fatal("diff_unavailable field should be present in review.json")
	} else if diffUnavailable != true {
		t.Errorf("diff_unavailable should be true, got %v", diffUnavailable)
	}

	// Check that findings are still present
	if specAlignment, ok := parsed["spec_alignment"]; !ok {
		t.Fatal("spec_alignment findings should be present in review.json")
	} else if specAlignmentList, ok := specAlignment.([]interface{}); !ok || len(specAlignmentList) != 1 {
		t.Errorf("expected 1 spec_alignment finding, got %v", specAlignment)
	}
}

func TestBundler_WriteReviewFindings_DiffUnavailableFalse(t *testing.T) {
	dir := t.TempDir()
	b := NewBundler(dir)
	if err := b.Init(); err != nil {
		t.Fatal(err)
	}

	findings := map[string][]review.Finding{
		"code_quality": {
			{Facet: "code_quality", Severity: review.SeverityWarning, File: "router.go", Line: 10, Description: "long function"},
		},
	}

	output := ReviewFindingsOutput{
		Findings:        findings,
		DiffUnavailable: false,
	}

	if err := b.WriteReviewFindings(output); err != nil {
		t.Fatalf("WriteReviewFindings: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil {
		t.Fatalf("read review.json: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("review.json should be valid JSON: %v", err)
	}

	// Check that diff_unavailable field is present and false
	if diffUnavailable, ok := parsed["diff_unavailable"]; !ok {
		t.Fatal("diff_unavailable field should be present in review.json")
	} else if diffUnavailable != false {
		t.Errorf("diff_unavailable should be false, got %v", diffUnavailable)
	}

	// Check that findings are still present
	if codeQuality, ok := parsed["code_quality"]; !ok {
		t.Fatal("code_quality findings should be present in review.json")
	} else if codeQualityList, ok := codeQuality.([]interface{}); !ok || len(codeQualityList) != 1 {
		t.Errorf("expected 1 code_quality finding, got %v", codeQuality)
	}
}
