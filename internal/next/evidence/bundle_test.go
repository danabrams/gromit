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

func TestMetrics_NormalizeNilFields(t *testing.T) {
	m := Metrics{}
	m.NormalizeNilFields()
	if m.Invocations == nil {
		t.Fatal("Invocations should not be nil after NormalizeNilFields")
	}
}
