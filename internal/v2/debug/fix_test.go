package debug

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyPlanWritesFile(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "foo.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	plan := FixPlan{
		Scope: ScopeLearnable,
		Edits: []FileEdit{
			{Path: "foo.txt", NewContent: []byte("updated")},
		},
	}

	result, err := ApplyPlan(root, plan)
	if err != nil {
		t.Fatalf("ApplyPlan error: %v", err)
	}
	if result.Scope != ScopeLearnable {
		t.Fatalf("scope = %v, want %v", result.Scope, ScopeLearnable)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("file content = %q, want %q", string(data), "updated")
	}
}

func TestApplyPlanSystemicIncludesRecommendation(t *testing.T) {
	root := t.TempDir()
	plan := FixPlan{
		Scope:          ScopeSystemic,
		Recommendation: "Consider refactoring the shared runtime guard",
		Edits: []FileEdit{
			{Path: "bar.txt", NewContent: []byte("systemic")},
		},
	}

	result, err := ApplyPlan(root, plan)
	if err != nil {
		t.Fatalf("ApplyPlan error: %v", err)
	}
	if result.Scope != ScopeSystemic {
		t.Fatalf("scope = %v, want %v", result.Scope, ScopeSystemic)
	}
	if result.RecommendHumanReview != plan.Recommendation {
		t.Fatalf("recommendation = %q, want %q", result.RecommendHumanReview, plan.Recommendation)
	}
	data, err := os.ReadFile(filepath.Join(root, "bar.txt"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(data) != "systemic" {
		t.Fatalf("bar.txt = %q, want %q", string(data), "systemic")
	}
}

func TestApplyPlanSystemicRecommendationRequired(t *testing.T) {
	root := t.TempDir()
	plan := FixPlan{
		Scope: ScopeSystemic,
		Edits: []FileEdit{
			{Path: "baz.txt", NewContent: []byte("needs review")},
		},
	}

	if _, err := ApplyPlan(root, plan); err == nil {
		t.Fatal("want error when systemic recommendation is missing")
	}
}
