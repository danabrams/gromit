package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
