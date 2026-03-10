package specreview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	githubllm "github.com/danabrams/gromit/internal/v2/llmtypes"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	stagedesc "github.com/danabrams/gromit/internal/v2/stage/names"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

func TestVerdictFromFindings(t *testing.T) {
	cases := []struct {
		name     string
		findings []SpecReviewFinding
		want     string
	}{
		{
			name:     "issue present",
			findings: []SpecReviewFinding{{Verdict: "issue"}},
			want:     "issue",
		},
		{
			name: "all pass",
			findings: []SpecReviewFinding{
				{Verdict: "pass"},
				{Verdict: "pass"},
			},
			want: "pass",
		},
		{
			name: "no findings",
			want: "pass",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := verdictFromFindings(tt.findings)
			if got != tt.want {
				t.Fatalf("%s: verdict = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestParseSpecReviewOutput(t *testing.T) {
	source := `{
        "findings": [
            {
                "verdict": "issue",
                "severity": "critical",
                "category": "quality",
                "scope": "prompt rendering",
                "description": "Missing validation in spec review",
                "affected_files": ["cmd/gromit/review_spec_validation_acceptance_test.go"]
            },
            {
                "verdict": "pass",
                "severity": "low",
                "category": "test_coverage",
                "scope": "test coverage",
                "description": "Tests look good",
                "affected_files": []
            }
        ],
        "summary": " Spec-level review summary. "
    }`

	artifacts, err := parseSpecReviewOutput(source)
	if err != nil {
		t.Fatalf("parse spec review output: %v", err)
	}

	if artifacts.Summary != "Spec-level review summary." {
		t.Fatalf("summary = %q, want %q", artifacts.Summary, "Spec-level review summary.")
	}
	if artifacts.Verdict != "issue" {
		t.Fatalf("verdict = %q, want issue", artifacts.Verdict)
	}
	if got := len(artifacts.Findings); got != 2 {
		t.Fatalf("findings count = %d, want 2", got)
	}

	first := artifacts.Findings[0]
	if first.Title != "prompt rendering" {
		t.Fatalf("first finding title = %q, want %q", first.Title, "prompt rendering")
	}
	if first.Description != "Missing validation in spec review" {
		t.Fatalf("description = %q", first.Description)
	}
	if first.Severity != stagepkg.SpecFindingSeverityCritical {
		t.Fatalf("severity = %q, want %q", first.Severity, stagepkg.SpecFindingSeverityCritical)
	}
	if first.Category != stagepkg.SpecFindingCategoryQuality {
		t.Fatalf("category = %q, want %q", first.Category, stagepkg.SpecFindingCategoryQuality)
	}
	if len(first.AffectedFiles) != 1 || first.AffectedFiles[0] != "cmd/gromit/review_spec_validation_acceptance_test.go" {
		t.Fatalf("affected files = %v", first.AffectedFiles)
	}
}

func TestNewSpecReviewStageRequiresConfig(t *testing.T) {
	if _, err := New(nil, nil, nil, nil, "", "", ""); err == nil {
		t.Fatal("expected error when config is nil")
	}

	cfg := &config.Config{Project: config.ProjectConfig{Profile: "example"}}
	stage, err := New(cfg, &fakeGitDiffer{}, &fakeLLMProvider{resp: &githubllm.LLMInvokeResponse{Success: true}}, nil, "", "", "")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	want := stagedesc.Describe("spec-review", cfg)
	if stage.Name() != want {
		t.Fatalf("stage name = %q, want %q", stage.Name(), want)
	}
}

func TestRunIncludesPlanAndDiffInPrompt(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planText := "Plan summary for spec review"
	planPath := filepath.Join(planDir, "plan.md")
	if err := os.WriteFile(planPath, []byte(planText), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	git := &fakeGitDiffer{diff: "@@ -0,0 +1 @@ +new feature"}
	provider := &fakeLLMProvider{resp: &githubllm.LLMInvokeResponse{
		Success: true,
		Output:  `{"findings": [{"verdict": "pass", "severity": "low", "category": "code_quality", "scope": "spec", "description": "looks good", "affected_files": ["cmd/spec.md"]}], "summary": "summary"}`,
	}}
	cfg := &config.Config{Paths: config.PathsConfig{GromitDir: ".gromit"}}
	stage, err := New(cfg, git, provider, nil, "", "", "")
	if err != nil {
		t.Fatalf("new stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "spec-123", Title: "Spec review", Description: "desc"},
		Worktree: root,
	}

	result, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", result.Decision)
	}
	artifacts, ok := result.Artifacts.(*SpecReviewArtifacts)
	if !ok || artifacts == nil {
		t.Fatalf("artifacts missing: %#v", result.Artifacts)
	}
	if artifacts.Summary != "summary" {
		t.Fatalf("summary = %q", artifacts.Summary)
	}
	if !strings.Contains(provider.lastRequest.Prompt, planText) {
		t.Fatalf("prompt missing plan: %q", provider.lastRequest.Prompt)
	}
	if !strings.Contains(provider.lastRequest.Prompt, git.diff) {
		t.Fatalf("prompt missing diff: %q", provider.lastRequest.Prompt)
	}
}

func TestRunFailsOnHighSeverityFinding(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte("plan"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	git := &fakeGitDiffer{diff: "diff"}
	provider := &fakeLLMProvider{resp: &githubllm.LLMInvokeResponse{
		Success: true,
		Output:  `{"findings": [{"verdict": "pass", "severity": "high", "category": "code_quality", "scope": "spec", "description": "critical", "affected_files": []}], "summary": "issues"}`,
	}}
	cfg := &config.Config{Paths: config.PathsConfig{GromitDir: ".gromit"}}
	stage, err := New(cfg, git, provider, nil, "", "", "")
	if err != nil {
		t.Fatalf("new stage: %v", err)
	}

	result, err := stage.Run(context.Background(), &stagepkg.Request{Bead: stagepkg.BeadInfo{ID: "spec"}, Worktree: root})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Decision != stagepkg.DecisionFail {
		t.Fatalf("decision = %v, want fail", result.Decision)
	}
}

func TestRunCreatesFromReviewBeads(t *testing.T) {
	root := t.TempDir()
	planDir := filepath.Join(root, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte("plan"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	git := &fakeGitDiffer{diff: "diff"}
	provider := &fakeLLMProvider{resp: &githubllm.LLMInvokeResponse{
		Success: true,
		Output: `{
			"verdict": "pass",
			"summary": "summary",
			"findings": [
				{
					"verdict": "pass",
					"severity": "low",
					"category": "quality",
					"scope": "spec",
					"description": "spec improvement",
					"affected_files": []
				},
				{
					"verdict": "pass",
					"severity": "low",
					"category": "quality",
					"scope": "general",
					"description": "general observation",
					"affected_files": []
				}
			]
		}`,
	}}

	tracker := newRecordingTaskTracker(
		&trackertypes.Bead{ID: "bead-spec", Title: "spec improvement"},
		&trackertypes.Bead{ID: "bead-general", Title: "general observation"},
	)

	cfg := &config.Config{Paths: config.PathsConfig{GromitDir: ".gromit"}}
	stage, err := New(cfg, git, provider, tracker, "", "", "")
	if err != nil {
		t.Fatalf("new stage: %v", err)
	}

	specID := "spec-123"
	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: specID, Title: "spec review"},
		Worktree: root,
	}
	result, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", result.Decision)
	}
	artifacts, ok := result.Artifacts.(*SpecReviewArtifacts)
	if !ok || artifacts == nil {
		t.Fatalf("artifacts missing: %#v", result.Artifacts)
	}

	if len(tracker.created) != 2 {
		t.Fatalf("created beads = %d, want 2", len(tracker.created))
	}

	specLabel := "spec:" + specID
	first := tracker.created[0]
	if len(first.Labels) != 2 || first.Labels[0] != "from-review" || first.Labels[1] != specLabel {
		t.Fatalf("first bead labels = %v, want from-review + %s", first.Labels, specLabel)
	}

	second := tracker.created[1]
	if len(second.Labels) != 1 || second.Labels[0] != "from-review" {
		t.Fatalf("second bead labels = %v, want [from-review]", second.Labels)
	}

	if len(artifacts.CreatedBeads) != 2 {
		t.Fatalf("created beads artifact = %d, want 2", len(artifacts.CreatedBeads))
	}
	for idx, bead := range artifacts.CreatedBeads {
		if bead == nil {
			t.Fatalf("CreatedBeads[%d] nil", idx)
		}
		if bead.ID != tracker.responses[idx].ID {
			t.Fatalf("CreatedBeads[%d].ID = %s, want %s", idx, bead.ID, tracker.responses[idx].ID)
		}
	}
}

func TestBeadTitleForFindingFallbacks(t *testing.T) {
	cases := []struct {
		name    string
		finding stagepkg.Finding
		want    string
	}{
		{
			name:    "description wins",
			finding: stagepkg.Finding{Description: "  explicit desc  "},
			want:    "explicit desc",
		},
		{
			name:    "category fallback",
			finding: stagepkg.Finding{Category: stagepkg.CategoryBug},
			want:    "Spec review: bug",
		},
		{
			name:    "scope fallback",
			finding: stagepkg.Finding{Scope: stagepkg.ScopeGeneral},
			want:    "Spec review: general finding",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := beadTitleForFinding(tt.finding); got != tt.want {
				t.Fatalf("bead title = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStage_RunFailsOnCriticalFinding(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	planDir := filepath.Join(root, defaultGromitDir, v2DirName)
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(planDir, planFileName), []byte("plan"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	git := &fakeGitDiffer{diff: "cumulative diff"}
	provider := &fakeLLMProvider{
		resp: &githubllm.LLMInvokeResponse{
			Success: true,
			Output:  `{"verdict": "reject", "findings": [{"severity": "critical", "category": "bug", "scope": "spec", "description": "blocking issue"}]}`,
		},
	}

	cfg := &config.Config{}
	stage, err := New(cfg, git, provider, nil, "", "", "")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := &stagepkg.Request{
		Bead:     stagepkg.BeadInfo{ID: "spec-2", Title: "Spec Review"},
		Worktree: root,
	}

	result, err := stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Decision != stagepkg.DecisionFail {
		t.Fatalf("Decision = %v, want Fail", result.Decision)
	}
}

type fakeGitDiffer struct {
	diff string
}

func (f *fakeGitDiffer) DiffFromBase(_ context.Context, _ string) (string, error) {
	return f.diff, nil
}

type fakeLLMProvider struct {
	lastRequest githubllm.LLMInvokeRequest
	resp        *githubllm.LLMInvokeResponse
}

func (f *fakeLLMProvider) Invoke(_ context.Context, req githubllm.LLMInvokeRequest) (*githubllm.LLMInvokeResponse, error) {
	f.lastRequest = req
	if f.resp == nil {
		return nil, nil
	}
	return f.resp, nil
}

func (fakeLLMProvider) StreamInvoke(_ context.Context, _ githubllm.LLMStreamInvokeRequest) (*githubllm.LLMInvokeResponse, error) {
	return nil, errors.New("not supported")
}

type recordingTaskTracker struct {
	created   []trackertypes.TaskTrackerCreateBeadRequest
	responses []*trackertypes.Bead
}

func newRecordingTaskTracker(responses ...*trackertypes.Bead) *recordingTaskTracker {
	return &recordingTaskTracker{responses: responses}
}

func (r *recordingTaskTracker) NextBead(_ context.Context, _ trackertypes.TaskTrackerNextBeadRequest) (*trackertypes.TaskTrackerNextBeadResponse, error) {
	return &trackertypes.TaskTrackerNextBeadResponse{}, nil
}

func (r *recordingTaskTracker) ShowBead(_ context.Context, _ string) (*trackertypes.Bead, error) {
	return nil, nil
}

func (r *recordingTaskTracker) CreateBead(_ context.Context, req trackertypes.TaskTrackerCreateBeadRequest) (*trackertypes.TaskTrackerCreateBeadResponse, error) {
	r.created = append(r.created, req)
	idx := len(r.created) - 1
	var bead *trackertypes.Bead
	if idx < len(r.responses) {
		bead = r.responses[idx]
	}
	return &trackertypes.TaskTrackerCreateBeadResponse{Bead: bead}, nil
}

func (r *recordingTaskTracker) CloseBead(_ context.Context, _ trackertypes.TaskTrackerCloseBeadRequest) (*trackertypes.TaskTrackerCloseBeadResponse, error) {
	return &trackertypes.TaskTrackerCloseBeadResponse{Closed: true}, nil
}

func (r *recordingTaskTracker) QueryBeads(_ context.Context, _ trackertypes.TaskTrackerQueryBeadsRequest) (*trackertypes.TaskTrackerQueryBeadsResponse, error) {
	return &trackertypes.TaskTrackerQueryBeadsResponse{}, nil
}

type fakeTaskTracker struct {
	requests []trackertypes.TaskTrackerCreateBeadRequest
}

func (f *fakeTaskTracker) NextBead(_ context.Context, _ trackertypes.TaskTrackerNextBeadRequest) (*trackertypes.TaskTrackerNextBeadResponse, error) {
	return nil, nil
}

func (f *fakeTaskTracker) ShowBead(_ context.Context, _ string) (*trackertypes.Bead, error) {
	return nil, nil
}

func (f *fakeTaskTracker) CreateBead(_ context.Context, req trackertypes.TaskTrackerCreateBeadRequest) (*trackertypes.TaskTrackerCreateBeadResponse, error) {
	f.requests = append(f.requests, req)
	return &trackertypes.TaskTrackerCreateBeadResponse{Bead: &trackertypes.Bead{ID: fmt.Sprintf("created-%d", len(f.requests))}}, nil
}

func (f *fakeTaskTracker) CloseBead(_ context.Context, _ trackertypes.TaskTrackerCloseBeadRequest) (*trackertypes.TaskTrackerCloseBeadResponse, error) {
	return nil, nil
}

func (f *fakeTaskTracker) QueryBeads(_ context.Context, _ trackertypes.TaskTrackerQueryBeadsRequest) (*trackertypes.TaskTrackerQueryBeadsResponse, error) {
	return &trackertypes.TaskTrackerQueryBeadsResponse{}, nil
}
