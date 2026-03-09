package specreview_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/specreview"
)

func TestSpecReview_PassWhenNoFindings(t *testing.T) {
	stage, _, _, request := setupStage(t, `{"verdict":"pass","findings":[]}`)
	result, err := stage.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want DecisionProceed", result.Decision)
	}
	artifacts, ok := result.Artifacts.(*specreview.SpecReviewArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T, want *specreview.SpecReviewArtifacts", result.Artifacts)
	}
	if artifacts.Verdict != "pass" {
		t.Fatalf("verdict = %q, want \"pass\"", artifacts.Verdict)
	}
	if len(artifacts.Findings) != 0 {
		t.Fatalf("findings = %d, want 0", len(artifacts.Findings))
	}
}

func TestSpecReview_FailWhenCriticalFinding(t *testing.T) {
	stage, _, _, request := setupStage(t, `{"verdict":"fail","findings":[{"severity":"critical","category":"bug","scope":"spec","description":"bad","affected_files":["foo.go"]}]}`)
	result, err := stage.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Decision != stagepkg.DecisionFail {
		t.Fatalf("decision = %v, want DecisionFail", result.Decision)
	}
	artifacts := result.Artifacts.(*specreview.SpecReviewArtifacts)
	if len(artifacts.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(artifacts.Findings))
	}
	if artifacts.Findings[0].Severity != stagepkg.FindingSeverityCritical {
		t.Fatalf("severity = %q, want critical", artifacts.Findings[0].Severity)
	}
}

func TestSpecReview_PassWithWarnings(t *testing.T) {
	stage, _, _, request := setupStage(t, `{"verdict":"pass","findings":[{"severity":"warning","category":"quality","scope":"general","description":"keep an eye on this","affected_files":["bar.go"]}]}`)
	result, err := stage.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want DecisionProceed", result.Decision)
	}
	artifacts := result.Artifacts.(*specreview.SpecReviewArtifacts)
	if len(artifacts.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(artifacts.Findings))
	}
}

func TestSpecReview_VerdictForcedFailOnCritical(t *testing.T) {
	stage, _, _, request := setupStage(t, `{"verdict":"pass","findings":[{"severity":"critical","category":"security","scope":"general","description":"vulnerability","affected_files":["baz.go"]}]}`)
	result, err := stage.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if result.Decision != stagepkg.DecisionFail {
		t.Fatalf("decision = %v, want DecisionFail", result.Decision)
	}
}

func TestSpecReview_DiffFromBaseCalledWithWorktree(t *testing.T) {
	stage, git, _, request := setupStage(t, `{"verdict":"pass","findings":[]}`)
	_, err := stage.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	if git.lastWorktree != request.Worktree {
		t.Fatalf("diff worktree = %q, want %q", git.lastWorktree, request.Worktree)
	}
}

func TestSpecReview_FindingsSurfacedInArtifacts(t *testing.T) {
	json := `{
        "verdict": "pass",
        "findings": [
            {
                "severity": "warning",
                "category": "architecture",
                "scope": "spec",
                "description": "mind the contract",
                "affected_files": ["combo.go"]
            }
        ]
    }`
	stage, _, _, request := setupStage(t, json)
	result, err := stage.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}
	artifacts := result.Artifacts.(*specreview.SpecReviewArtifacts)
	if len(artifacts.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(artifacts.Findings))
	}
	finding := artifacts.Findings[0]
	if finding.Category != stagepkg.FindingCategoryArchitecture {
		t.Fatalf("category = %q, want architecture", finding.Category)
	}
	if finding.Scope != stagepkg.FindingScopeSpec {
		t.Fatalf("scope = %q, want spec", finding.Scope)
	}
	if finding.Description != "mind the contract" {
		t.Fatalf("description = %q, want \"mind the contract\"", finding.Description)
	}
	if len(finding.AffectedFiles) != 1 || finding.AffectedFiles[0] != "combo.go" {
		t.Fatalf("affected files = %v, want [combo.go]", finding.AffectedFiles)
	}
}

func setupStage(t *testing.T, output string) (*specreview.Stage, *fakeGit, *fakeLLM, *stagepkg.StageRequest) {
	t.Helper()
	git := &fakeGit{diff: "sample diff"}
	llm := &fakeLLM{response: output}
	stage, err := specreview.New(git, llm, "fragment")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	request := &stagepkg.StageRequest{
		Worktree: t.TempDir(),
		Tier:     "high",
	}
	return stage, git, llm, request
}

type fakeGit struct {
	lastWorktree string
	diff         string
}

func (f *fakeGit) DiffFromBase(ctx context.Context, worktree string) (string, error) {
	f.lastWorktree = worktree
	return f.diff, nil
}

type fakeLLM struct {
	response string
}

func (f *fakeLLM) Invoke(ctx context.Context, req llmtypes.LLMInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return &llmtypes.LLMInvokeResponse{Success: true, Output: f.response}, nil
}

func (f *fakeLLM) StreamInvoke(ctx context.Context, req llmtypes.LLMStreamInvokeRequest) (*llmtypes.LLMInvokeResponse, error) {
	return nil, fmt.Errorf("stream not implemented")
}
