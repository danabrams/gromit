package decompose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestRunErrorsWhenPlanMissing(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Paths: config.PathsConfig{GromitDir: ".gromit"},
	}

	stg, err := New(cfg, &fakeLLM{}, &fakeTracker{})
	if err != nil {
		t.Fatalf("unexpected stage creation error: %v", err)
	}

	_, err = stg.Run(context.Background(), &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "spec"},
		Config: cfg,
	})
	if err == nil {
		t.Fatal("expected error when plan is missing")
	}
	if !strings.Contains(err.Error(), "plan not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunCreatesBeadsFromPlan(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planPath := filepath.Join(planDir, "plan.md")
	planContent := "# Plan\nTask 1: Do the thing"
	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llm := &fakeLLM{
		responses: []*llm.LLMResponse{
			{
				Success: true,
				Output: `[
                    {
                        "title": "first",
                        "description": "desc1",
                        "priority": "P1",
                        "estimated_files": 2,
                        "acceptance_criteria": ["crit1"],
                        "expected_outputs": ["out1"],
                        "depends_on_index": []
                    },
                    {
                        "title": "second",
                        "description": "desc2",
                        "priority": "P2",
                        "acceptance_criteria": ["crit2"],
                        "expected_outputs": ["out2"],
                        "depends_on_index": [0]
                    }
                ]`,
			},
		},
	}
	tracker := &fakeTracker{}
	stage, err := New(cfg, llm, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	res, err := stage.Run(context.Background(), &stagepkg.Request{
		Bead:   stagepkg.BeadInfo{ID: "spec"},
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if res.Decision != stagepkg.DecisionProceed {
		t.Fatalf("decision = %v, want proceed", res.Decision)
	}

	artifacts, ok := res.Artifacts.(*stagepkg.DecomposeArtifacts)
	if !ok {
		t.Fatalf("artifacts type = %T", res.Artifacts)
	}
	if len(artifacts.Beads) != 2 {
		t.Fatalf("expected 2 beads, got %d", len(artifacts.Beads))
	}
	if len(tracker.calls) != 2 {
		t.Fatalf("expected 2 create calls, got %d", len(tracker.calls))
	}
	if len(tracker.calls[1].Dependencies) == 0 || tracker.calls[1].Dependencies[0] != "bead-1" {
		t.Fatalf("second bead dependencies = %v", tracker.calls[1].Dependencies)
	}
	if !contains(tracker.calls[0].Labels, "spec:spec") {
		t.Fatalf("missing spec label")
	}
}

// TestRunIncrementsGenerationWhenRemediation verifies that Remediation=true increments the gen label.
// Gap-scoped prompt behavior is tested in TestRunUsesGapScopedPromptWhenGapAnalysisProvided
// and TestRunReadsGapAnalysisFromDiskWhenFieldEmpty.
func TestRunIncrementsGenerationWhenRemediation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	gapContent := "remediation plan content"
	planPath := filepath.Join(planDir, "plan.md")
	if err := os.WriteFile(planPath, []byte(gapContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llm := &fakeLLM{
		responses: []*llm.LLMResponse{{Success: true, Output: `[
			{
				"title": "gap",
				"description": "gap desc",
				"priority": "P1",
				"acceptance_criteria": ["crit"],
				"expected_outputs": ["out"],
				"depends_on_index": []
			},
			{
				"title": "gap followup",
				"description": "gap desc 2",
				"priority": "P1",
				"acceptance_criteria": ["crit2"],
				"expected_outputs": ["out2"],
				"depends_on_index": [0]
			}
		]`}},
	}
	tracker := &fakeTracker{}
	stage, err := New(cfg, llm, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead: stagepkg.BeadInfo{
			ID:     "spec",
			Labels: []string{"gen:0"},
		},
		Config:      cfg,
		Remediation: true,
	}

	_, err = stage.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}
	if len(llm.calls) == 0 {
		t.Fatal("expected llm to be invoked")
	}
	if !strings.Contains(llm.calls[0].Prompt, gapContent) {
		t.Fatalf("prompt missing gap content: %s", llm.calls[0].Prompt)
	}
	if !contains(tracker.calls[0].Labels, "gen:1") {
		t.Fatalf("expected generation label 1, got %v", tracker.calls[0].Labels)
	}
}

func TestNormalizeMaxValidationRetries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input int
		want  int
	}{
		{name: "negative returns zero", input: -3, want: 0},
		{name: "zero stays zero", input: 0, want: 0},
		{name: "positive preserved", input: 5, want: 5},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeMaxValidationRetries(tc.input); got != tc.want {
				t.Fatalf("normalizeMaxValidationRetries(%d) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestWithPromptTemplateOverridesDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Paths: config.PathsConfig{GromitDir: ".gromit"},
	}

	customTemplate := "Custom decompose: %s %s %s spec:%s"
	stg, err := New(cfg, &fakeLLM{}, &fakeTracker{}, WithPromptTemplate(customTemplate))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stg.promptTemplate != customTemplate {
		t.Fatalf("expected custom template, got: %q", stg.promptTemplate)
	}
}

func TestWithPromptTemplateIgnoresEmpty(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Paths: config.PathsConfig{GromitDir: ".gromit"},
	}

	stg, err := New(cfg, &fakeLLM{}, &fakeTracker{}, WithPromptTemplate("   "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stg.promptTemplate != defaultDecomposePromptTemplate {
		t.Fatalf("expected default template when given whitespace, got: %q", stg.promptTemplate)
	}
}

func TestRunUsesGapScopedPromptWhenGapAnalysisProvided(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planContent := "# Full Plan\nTask 1: Build widget\nTask 2: Build gadget"
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	gapContent := "Criterion 3 failed: widgets don't commit events"

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llmFake := &fakeLLM{
		responses: []*llm.LLMResponse{{Success: true, Output: `[
			{
				"title": "fix widget commits",
				"description": "add event commits to widgets",
				"priority": "P1",
				"acceptance_criteria": ["widgets commit events"],
				"expected_outputs": ["commit after widget stage"],
				"covers_tasks": [1],
				"depends_on_index": []
			},
			{
				"title": "verify widget events",
				"description": "add tests for widget event commits",
				"priority": "P1",
				"acceptance_criteria": ["widget event tests pass"],
				"expected_outputs": ["widget event test file"],
				"covers_tasks": [2],
				"depends_on_index": [0]
			}
		]`}},
	}
	tracker := &fakeTracker{}
	stg, err := New(cfg, llmFake, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:        stagepkg.BeadInfo{ID: "spec", Labels: []string{"gen:0"}},
		Config:      cfg,
		Remediation: true,
		GapAnalysis: gapContent,
	}

	_, err = stg.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if len(llmFake.calls) == 0 {
		t.Fatal("expected llm invocation")
	}
	prompt := llmFake.calls[0].Prompt
	if !strings.Contains(prompt, planContent) {
		t.Fatal("prompt missing plan content for context")
	}
	if !strings.Contains(prompt, gapContent) {
		t.Fatal("prompt missing gap analysis content for scoping")
	}
	if !strings.Contains(prompt, "ONLY create beads") {
		t.Fatal("prompt missing gap-scoping instruction")
	}
}

func TestRunReadsGapAnalysisFromDiskWhenFieldEmpty(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	planContent := "# Plan\nTask 1: Build thing"
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	gapContent := "Criterion 5 failed: retries not preserved"
	if err := os.WriteFile(filepath.Join(planDir, "gap-analysis.md"), []byte(gapContent), 0o644); err != nil {
		t.Fatalf("write gap file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llmFake := &fakeLLM{
		responses: []*llm.LLMResponse{{Success: true, Output: `[
			{
				"title": "preserve retries",
				"description": "keep retry commits",
				"priority": "P1",
				"acceptance_criteria": ["retries preserved"],
				"expected_outputs": ["separate retry commits"],
				"covers_tasks": [1],
				"depends_on_index": []
			},
			{
				"title": "verify retry preservation",
				"description": "add tests for retry preservation",
				"priority": "P1",
				"acceptance_criteria": ["retry tests pass"],
				"expected_outputs": ["retry test file"],
				"covers_tasks": [1],
				"depends_on_index": [0]
			}
		]`}},
	}
	tracker := &fakeTracker{}
	stg, err := New(cfg, llmFake, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:        stagepkg.BeadInfo{ID: "spec", Labels: []string{"gen:0"}},
		Config:      cfg,
		Remediation: true,
		// GapAnalysis intentionally empty — should fall back to disk
	}

	_, err = stg.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	prompt := llmFake.calls[0].Prompt
	if !strings.Contains(prompt, gapContent) {
		t.Fatal("prompt missing gap content from disk fallback")
	}
	if !strings.Contains(prompt, "ONLY create beads") {
		t.Fatal("prompt missing gap-scoping instruction from disk fallback")
	}
}

func TestRunUsesFindingsPromptWhenFindingsProvided(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	planDir := filepath.Join(tmpDir, ".gromit", "v2")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("mkdir plan dir: %v", err)
	}
	planContent := "# Plan\nTask 1: Fix beans"
	if err := os.WriteFile(filepath.Join(planDir, "plan.md"), []byte(planContent), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}

	cfg := &config.Config{
		ProjectRoot: tmpDir,
		Paths:       config.PathsConfig{GromitDir: ".gromit"},
	}

	llmFake := &fakeLLM{
		responses: []*llm.LLMResponse{{Success: true, Output: `[
			{
				"title": "fix beans",
				"description": "add bean coverage",
				"priority": "P1",
				"acceptance_criteria": ["bean tests"],
				"expected_outputs": ["bean test file"],
				"covers_tasks": [1],
				"depends_on_index": []
			},
			{
				"title": "verify beans",
				"description": "add bean verification",
				"priority": "P1",
				"acceptance_criteria": ["bean verification"],
				"expected_outputs": ["bean verification file"],
				"covers_tasks": [1],
				"depends_on_index": [0]
			}
		]`}},
	}
	tracker := &fakeTracker{}
	stg, err := New(cfg, llmFake, tracker)
	if err != nil {
		t.Fatalf("create stage: %v", err)
	}

	req := &stagepkg.Request{
		Bead:        stagepkg.BeadInfo{ID: "spec", Labels: []string{"gen:0"}},
		Config:      cfg,
		Remediation: true,
		Findings: []stagepkg.Finding{
			{
				Severity:      stagepkg.SeverityCritical,
				Category:      stagepkg.CategoryQuality,
				Scope:         stagepkg.ScopeSpec,
				Description:   "Beans are not covered by automated tests",
				AffectedFiles: []string{"docs/beans.md"},
			},
		},
	}

	if _, err := stg.Run(context.Background(), req); err != nil {
		t.Fatalf("run stage: %v", err)
	}

	if len(llmFake.calls) == 0 {
		t.Fatal("expected llm invocation")
	}
	prompt := llmFake.calls[0].Prompt
	if !strings.Contains(prompt, planContent) {
		t.Fatal("prompt missing plan content for context")
	}
	if !strings.Contains(prompt, "## Findings to Fix") {
		t.Fatal("expected findings template header to appear")
	}
	if !strings.Contains(prompt, "Create one or more beads that specifically address the findings above") {
		t.Fatal("prompt missing findings instructions")
	}
	if !strings.Contains(prompt, "Affected files: docs/beans.md") {
		t.Fatal("prompt missing affected files list")
	}
}

func TestFormatFindingsIncludesSeverityCategoryScopeDescriptionAndAffectedFiles(t *testing.T) {
	t.Parallel()
	req := &stagepkg.Request{
		Findings: []stagepkg.Finding{
			{
				Severity:      stagepkg.SeverityCritical,
				Category:      stagepkg.CategoryBug,
				Scope:         stagepkg.ScopeSpec,
				Description:   "Beans need tests",
				AffectedFiles: []string{"beans.go", "beans_test.go"},
			},
		},
	}

	out := formatFindings(req)
	if !strings.Contains(out, "### critical — bug (spec)") {
		t.Fatalf("missing severity/category/scope header: %q", out)
	}
	if !strings.Contains(out, "Beans need tests") {
		t.Fatalf("missing description: %q", out)
	}
	if !strings.Contains(out, "Affected files: beans.go, beans_test.go") {
		t.Fatalf("missing affected files list: %q", out)
	}
}

func TestDecomposePromptContainsBehavioralCriteriaInstruction(t *testing.T) {
	t.Parallel()
	if !strings.Contains(defaultDecomposePromptTemplate, "observable behavior") {
		t.Fatal("default decompose prompt must instruct behavioral acceptance criteria")
	}
	if !strings.Contains(defaultDecomposePromptTemplate, "NOT a file path") {
		t.Fatal("default decompose prompt must warn against file-path criteria")
	}
}

func TestRemediationDecomposePromptContainsBehavioralCriteriaInstruction(t *testing.T) {
	t.Parallel()
	if !strings.Contains(remediationDecomposePromptTemplate, "observable behavior") {
		t.Fatal("remediation decompose prompt must instruct behavioral acceptance criteria")
	}
}

func TestFindingsDecomposePromptMentionsTargetedFixBeadsAndDependencies(t *testing.T) {
	t.Parallel()
	if !strings.Contains(findingsDecomposePromptTemplate, "Create exactly one targeted fix bead per finding.") {
		t.Fatal("findings prompt must direct a single fix bead per finding")
	}
	if !strings.Contains(findingsDecomposePromptTemplate, "Always set depends_on_index when shared affected files appear in multiple findings.") {
		t.Fatal("findings prompt must instruct linking dependencies when findings share affected files")
	}
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

type fakeLLM struct {
	responses []*llm.LLMResponse
	calls     []llm.InvokeRequest
}

func (f *fakeLLM) Invoke(ctx context.Context, req llm.InvokeRequest) (*llm.LLMResponse, error) {
	f.calls = append(f.calls, req)
	if len(f.responses) == 0 {
		return nil, fmt.Errorf("no responses")
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func (fakeLLM) StreamInvoke(ctx context.Context, req llm.StreamInvokeRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Success: true, Output: "[]"}, nil
}

type fakeTracker struct {
	calls  []createCall
	nextID int
}

type createCall struct {
	Title        string
	Description  string
	Priority     int
	Labels       []string
	Dependencies []string
}

func (f *fakeTracker) NextBead(ctx context.Context, req tasktracker.NextBeadRequest) (*tasktracker.NextBeadResponse, error) {
	return &tasktracker.NextBeadResponse{}, nil
}

func (f *fakeTracker) ShowBead(context.Context, string) (*tasktracker.Bead, error) {
	return nil, nil
}

func (f *fakeTracker) CreateBead(ctx context.Context, req tasktracker.CreateBeadRequest) (*tasktracker.CreateBeadResponse, error) {
	f.nextID++
	id := fmt.Sprintf("bead-%d", f.nextID)
	call := createCall{
		Title:        req.Title,
		Description:  req.Description,
		Priority:     req.Priority,
		Labels:       append([]string(nil), req.Labels...),
		Dependencies: append([]string(nil), req.Dependencies...),
	}
	f.calls = append(f.calls, call)
	return &tasktracker.CreateBeadResponse{Bead: &tasktracker.Bead{
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Labels:      append([]string(nil), call.Labels...),
		DependsOn:   append([]string(nil), req.Dependencies...),
	}}, nil
}

func (f *fakeTracker) CloseBead(ctx context.Context, req tasktracker.CloseBeadRequest) (*tasktracker.CloseBeadResponse, error) {
	return &tasktracker.CloseBeadResponse{Closed: true}, nil
}

func (f *fakeTracker) QueryBeads(ctx context.Context, req tasktracker.QueryBeadsRequest) (*tasktracker.QueryBeadsResponse, error) {
	return &tasktracker.QueryBeadsResponse{}, nil
}
