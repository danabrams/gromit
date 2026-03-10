package loop

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
	"github.com/danabrams/gromit/internal/v2/stage/finding"
	stageaccept "github.com/danabrams/gromit/internal/v2/stage/accept"
	stagepresent "github.com/danabrams/gromit/internal/v2/stage/present"
	"github.com/danabrams/gromit/internal/v2/stage/specreview"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

const (
	specReviewSeverityWarning    = stagepkg.SpecFindingSeverity("warning")
	specReviewSeveritySuggestion = stagepkg.SpecFindingSeverity("suggestion")
)

func TestSpecLoopEnsureAcceptanceAndReviewCreatesFromReviewBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-review-loop"

	tracker := newRecordingTaskTracker()
	acceptStage := newFakeAcceptStage()

	reviewFindings := []specreview.SpecReviewFinding{
		{
			Severity:    specReviewSeverityWarning,
			Category:    stagepkg.SpecFindingCategoryQuality,
			Scope:       stagepkg.SpecFindingScopeSpec,
			Description: "spec scoped warning",
		},
		{
			Severity:    specReviewSeveritySuggestion,
			Category:    stagepkg.SpecFindingCategoryQuality,
			Scope:       stagepkg.SpecFindingScopeStage,
			Description: "general suggestion",
		},
	}

	specReviewStage := newScriptedSpecReviewStage(&stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &specreview.SpecReviewArtifacts{
			Verdict:  "pass",
			Findings: reviewFindings,
		},
	})

	s := &SpecLoop{
		adapters:        adapter.AdapterSet{TaskTracker: tracker},
		acceptStage:     acceptStage,
		specReviewStage: specReviewStage,
	}

	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Worktree: "worktree"}
	if _, _, err := s.ensureAcceptance(ctx, &req, specID); err != nil {
		t.Fatalf("ensure acceptance and review: %v", err)
	}

	if got, want := len(tracker.created), len(reviewFindings); got != want {
		t.Fatalf("created beads = %d, want %d", got, want)
	}

	for idx, created := range tracker.created {
		finding := reviewFindings[idx]
		wantLabels := []string{"from-review"}
		if finding.Scope == stagepkg.SpecFindingScopeSpec {
			wantLabels = append(wantLabels, "spec:"+specID)
		}
		if !reflect.DeepEqual(created.Labels, wantLabels) {
			t.Fatalf("labels for finding[%d] = %v, want %v", idx, created.Labels, wantLabels)
		}
	}
}

func TestSpecLoopPassWithImprovementsUsesMediumSeverityFindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-review-medium"

	tracker := newRecordingTaskTracker()
	acceptStage := newFakeAcceptStage()

	reviewArtifact := &specreview.SpecReviewArtifacts{
		Verdict: "pass",
		Findings: []specreview.SpecReviewFinding{ {
			Severity:    stagepkg.SpecFindingSeverityMedium,
			Category:    stagepkg.SpecFindingCategoryQuality,
			Scope:       stagepkg.SpecFindingScopeSpec,
			Description: "medium improvement",
		}},
	}
	specReviewStage := newScriptedSpecReviewStage(&stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: reviewArtifact,
	})

	s := &SpecLoop{
		adapters:        adapter.AdapterSet{TaskTracker: tracker},
		acceptStage:     acceptStage,
		specReviewStage: specReviewStage,
	}
	req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Worktree: "worktree"}
	if _, _, err := s.ensureAcceptance(ctx, &req, specID); err != nil {
		t.Fatalf("ensure acceptance and review: %v", err)
	}

	if got, want := len(tracker.created), 1; got != want {
		t.Fatalf("created beads = %d, want %d", got, want)
	}

	created := tracker.created[0]
	if created.Description != "medium improvement" {
		t.Fatalf("created description = %q, want %q", created.Description, "medium improvement")
	}
	wantLabels := []string{"from-review", "spec:" + specID}
	if !reflect.DeepEqual(created.Labels, wantLabels) {
		t.Fatalf("created labels = %v, want %v", created.Labels, wantLabels)
	}
}

func TestSpecLoopCreateFromReviewBeadsIgnoresCriticalFindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-review-loop"

	tracker := newRecordingTaskTracker()
	s := &SpecLoop{adapters: adapter.AdapterSet{TaskTracker: tracker}}

	findings := []finding.Finding{
		{
			Severity:    finding.SeverityCritical,
			Category:    finding.CategoryQuality,
			Scope:       string(stagepkg.SpecFindingScopeSpec),
			Description: "critical",
		},
		{
			Severity:    finding.SeverityWarning,
			Category:    finding.CategoryQuality,
			Scope:       string(stagepkg.SpecFindingScopeSpec),
			Description: "non-critical",
		},
	}

	if err := s.createFromReviewBeads(ctx, specID, findings); err != nil {
		t.Fatalf("create from-review beads: %v", err)
	}

	if got, want := len(tracker.created), 1; got != want {
		t.Fatalf("created beads = %d, want %d", got, want)
	}

	created := tracker.created[0]
	if got, want := created.Description, "non-critical"; got != want {
		t.Fatalf("created bead description = %q, want %q", got, want)
	}

	wantLabels := []string{"from-review", "spec:" + specID}
	if !reflect.DeepEqual(created.Labels, wantLabels) {
		t.Fatalf("created labels = %v, want %v", created.Labels, wantLabels)
	}
}

func TestSpecLoopPostBeadPipelineAcceptFailureCapturesFindings(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-post-bead-accept-fail"

	acceptFinding := stagepkg.SpecFinding{
		Title:       "missing nil guard",
		Description: "The acceptance criterion requires a nil check before reading the payload.",
		Severity:    stagepkg.SpecFindingSeverityHigh,
		Category:    stagepkg.SpecFindingCategoryAcceptance,
		Scope:       stagepkg.SpecFindingScopeSpec,
	}
	acceptStage := newScriptedAcceptStage(
		stagepkg.Result{
			Decision: stagepkg.DecisionFail,
			Artifacts: &stageaccept.AcceptArtifacts{
				Findings: []stagepkg.SpecFinding{acceptFinding},
			},
		},
		stagepkg.Result{
			Decision:  stagepkg.DecisionProceed,
			Artifacts: &stageaccept.AcceptArtifacts{},
		},
	)

	reviewFinding := specreview.SpecReviewFinding{
		Title:         "retry path missing",
		Description:   "The handler never retries when the downstream call times out.",
		Severity:      stagepkg.SpecFindingSeverityWarning,
		Category:      stagepkg.SpecFindingCategoryQuality,
		Scope:         stagepkg.SpecFindingScopeSpec,
		AffectedFiles: []string{"internal/v2/loop/spec_loop.go"},
	}
	failureResult := stagepkg.Result{
		Decision: stagepkg.DecisionFail,
		Artifacts: &specreview.SpecReviewArtifacts{
			Verdict:  "issue",
			Findings: []specreview.SpecReviewFinding{reviewFinding},
		},
	}
	passResult := stagepkg.Result{
		Decision:  stagepkg.DecisionProceed,
		Artifacts: &specreview.SpecReviewArtifacts{Verdict: "pass"},
	}
	specReviewStage := newFakeSpecReviewStage(failureResult, passResult, passResult, passResult)

	remediationRunner := &recordingRemediationRunner{}

	loop, presentStage, _ := buildPostBeadLoopSpecLoop(t, specID, acceptStage, specReviewStage, remediationRunner, nil)
	if err := loop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	if remediationRunner.calls != 1 {
		t.Fatalf("remediation calls = %d, want 1", remediationRunner.calls)
	}
	if len(remediationRunner.lastFindings) != 2 {
		t.Fatalf("findings = %+v, want accept+review findings", remediationRunner.lastFindings)
	}
	if remediationRunner.lastFindings[0].Title != acceptFinding.Title {
		t.Fatalf("accept finding title = %q, want %q", remediationRunner.lastFindings[0].Title, acceptFinding.Title)
	}
	if remediationRunner.lastFindings[1].Title != reviewFinding.Title {
		t.Fatalf("review finding title = %q, want %q", remediationRunner.lastFindings[1].Title, reviewFinding.Title)
	}
	if !presentStage.called {
		t.Fatal("present stage should run after remediation completes")
	}
}

func TestSpecLoopPostBeadPipelineSpecReviewFailureSkipsPresent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-post-bead-review-fail"

	acceptStage := newFakeAcceptStage()

	failureFinding := specreview.SpecReviewFinding{
		Title:       "critical review issue",
		Description: "A blocking security concern was found.",
		Severity:    stagepkg.SpecFindingSeverityCritical,
		Category:    stagepkg.SpecFindingCategorySecurity,
		Scope:       stagepkg.SpecFindingScopeSpec,
	}
	specReviewStage := newFakeSpecReviewStage(stagepkg.Result{
		Decision: stagepkg.DecisionFail,
		Artifacts: &specreview.SpecReviewArtifacts{
			Verdict:  "issue",
			Findings: []specreview.SpecReviewFinding{failureFinding},
		},
	})

	loop, presentStage, _ := buildPostBeadLoopSpecLoop(t, specID, acceptStage, specReviewStage, nil, nil)
	if err := loop.Run(ctx, specID, nil); err == nil || !strings.Contains(err.Error(), "spec review failed") {
		t.Fatalf("run spec loop error = %v, want spec review failed", err)
	}
	if presentStage.called {
		t.Fatal("present stage should not run when spec review fails")
	}
}

func TestSpecLoopPostBeadPipelineWarningsCreateReviewBeads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-post-bead-warnings"

	acceptStage := newFakeAcceptStage()

	specScopeFinding := specreview.SpecReviewFinding{
		Title:         "document missing tests",
		Description:   "No automated tests cover the new parsing logic.",
		Verdict:       "pass",
		Severity:      stagepkg.SpecFindingSeverityWarning,
		Category:      stagepkg.SpecFindingCategoryTestGap,
		Scope:         stagepkg.SpecFindingScopeSpec,
		AffectedFiles: []string{"internal/v2/loop/spec_loop.go"},
	}
	stageScopeFinding := specreview.SpecReviewFinding{
		Title:         "cleanup doc typo",
		Description:   "The onboarding doc has a typo about the new flag.",
		Verdict:       "pass",
		Severity:      stagepkg.SpecFindingSeveritySuggestion,
		Category:      stagepkg.SpecFindingCategoryArchitecture,
		Scope:         stagepkg.SpecFindingScopeStage,
		AffectedFiles: []string{"docs/README.md"},
	}

	makePassResult := func() stagepkg.Result {
		return stagepkg.Result{
			Decision: stagepkg.DecisionProceed,
			Artifacts: &specreview.SpecReviewArtifacts{
				Verdict:  "pass",
				Findings: []specreview.SpecReviewFinding{specScopeFinding, stageScopeFinding},
			},
		}
	}
	specReviewStage := newFakeSpecReviewStage(makePassResult(), makePassResult(), makePassResult(), makePassResult())

	taskTracker := newRecordingTaskTracker()
	loop, presentStage, _ := buildPostBeadLoopSpecLoop(t, specID, acceptStage, specReviewStage, nil, taskTracker)
	if err := loop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}
	if !presentStage.called {
		t.Fatal("present stage should run when warnings do not block")
	}
	if got, want := len(taskTracker.created), 2; got != want {
		t.Fatalf("created beads = %d, want %d", got, want)
	}
	specLabel := tracker.SpecLabelFor(specID)
	specBead := taskTracker.created[0]
	if !containsLabel(specBead.Labels, "from-review") {
		t.Fatalf("spec bead missing from-review label: %v", specBead.Labels)
	}
	if !containsLabel(specBead.Labels, specLabel) {
		t.Fatalf("spec bead missing spec label: %v", specBead.Labels)
	}
	stageBead := taskTracker.created[1]
	if !containsLabel(stageBead.Labels, "from-review") {
		t.Fatalf("stage bead missing from-review label: %v", stageBead.Labels)
	}
	if containsLabel(stageBead.Labels, specLabel) {
		t.Fatalf("stage bead unexpectedly contains spec label: %v", stageBead.Labels)
	}
}

func TestSpecLoopPostBeadPipelineImprovementsUseTitleFallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-post-bead-improvements"

	acceptStage := newFakeAcceptStage()

	specFinding := specreview.SpecReviewFinding{
		Title:       "document spec improvements",
		Description: "",
		Severity:    specReviewSeverityWarning,
		Category:    stagepkg.SpecFindingCategoryQuality,
		Scope:       stagepkg.SpecFindingScopeSpec,
	}
	stageFinding := specreview.SpecReviewFinding{
		Title:       "general cleanup opportunity",
		Description: " ",
		Severity:    specReviewSeveritySuggestion,
		Category:    stagepkg.SpecFindingCategoryArchitecture,
		Scope:       stagepkg.SpecFindingScopeStage,
	}

	passResult := stagepkg.Result{
		Decision: stagepkg.DecisionProceed,
		Artifacts: &specreview.SpecReviewArtifacts{
			Verdict:  "pass",
			Findings: []specreview.SpecReviewFinding{specFinding, stageFinding},
		},
	}
	specReviewStage := newFakeSpecReviewStage(passResult, passResult)

	taskTracker := newRecordingTaskTracker()
	loop, presentStage, _ := buildPostBeadLoopSpecLoop(t, specID, acceptStage, specReviewStage, nil, taskTracker)
	if err := loop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}
	if !presentStage.called {
		t.Fatal("present stage should run after improvements")
	}
	if got, want := len(taskTracker.created), 2; got != want {
		t.Fatalf("created beads = %d, want %d", got, want)
	}

	specLabel := tracker.SpecLabelFor(specID)
	specBead := taskTracker.created[0]
	if specBead.Title != "document spec improvements" {
		t.Fatalf("spec bead title = %q, want %q", specBead.Title, "document spec improvements")
	}
	if specBead.Description != "document spec improvements" {
		t.Fatalf("spec bead description = %q, want %q", specBead.Description, "document spec improvements")
	}
	if !containsLabel(specBead.Labels, "from-review") {
		t.Fatalf("spec bead missing from-review label: %v", specBead.Labels)
	}
	if !containsLabel(specBead.Labels, specLabel) {
		t.Fatalf("spec bead missing spec label: %v", specBead.Labels)
	}

	stageBead := taskTracker.created[1]
	if stageBead.Title != "general cleanup opportunity" {
		t.Fatalf("stage bead title = %q, want %q", stageBead.Title, "general cleanup opportunity")
	}
	if stageBead.Description != "general cleanup opportunity" {
		t.Fatalf("stage bead description = %q, want %q", stageBead.Description, "general cleanup opportunity")
	}
	if !containsLabel(stageBead.Labels, "from-review") {
		t.Fatalf("stage bead missing from-review label: %v", stageBead.Labels)
	}
	if containsLabel(stageBead.Labels, specLabel) {
		t.Fatalf("stage bead unexpectedly contains spec label: %v", stageBead.Labels)
	}
}

func TestSpecLoopPostBeadPipelineWithNilSpecReviewStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	specID := "spec-post-bead-nil-review"

	acceptStage := newFakeAcceptStage()
	loop, presentStage, _ := buildPostBeadLoopSpecLoop(t, specID, acceptStage, nil, nil, nil)
	loop.specReviewStage = nil

	if err := loop.Run(ctx, specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}
	if !presentStage.called {
		t.Fatal("present stage should run when spec review stage is absent")
	}
}

// recordingTaskTracker captures create requests for verification.
type recordingTaskTracker struct {
	created []trackertypes.TaskTrackerCreateBeadRequest
}

func newRecordingTaskTracker() *recordingTaskTracker {
	return &recordingTaskTracker{}
}

func (r *recordingTaskTracker) NextBead(_ context.Context, _ trackertypes.TaskTrackerNextBeadRequest) (*trackertypes.TaskTrackerNextBeadResponse, error) {
	return &trackertypes.TaskTrackerNextBeadResponse{}, nil
}

func (r *recordingTaskTracker) ShowBead(_ context.Context, _ string) (*trackertypes.Bead, error) {
	return nil, nil
}

func (r *recordingTaskTracker) CreateBead(_ context.Context, req trackertypes.TaskTrackerCreateBeadRequest) (*trackertypes.TaskTrackerCreateBeadResponse, error) {
	r.created = append(r.created, req)
	return &trackertypes.TaskTrackerCreateBeadResponse{}, nil
}

func (r *recordingTaskTracker) CloseBead(_ context.Context, _ trackertypes.TaskTrackerCloseBeadRequest) (*trackertypes.TaskTrackerCloseBeadResponse, error) {
	return &trackertypes.TaskTrackerCloseBeadResponse{Closed: true}, nil
}

func (r *recordingTaskTracker) QueryBeads(_ context.Context, _ trackertypes.TaskTrackerQueryBeadsRequest) (*trackertypes.TaskTrackerQueryBeadsResponse, error) {
	return &trackertypes.TaskTrackerQueryBeadsResponse{}, nil
}

// scriptedSpecReviewStage allows tests to predefine results in sequence.
type scriptedSpecReviewStage struct {
	results []*stagepkg.Result
	calls   int
}

func newScriptedSpecReviewStage(results ...*stagepkg.Result) *scriptedSpecReviewStage {
	copied := append([]*stagepkg.Result(nil), results...)
	return &scriptedSpecReviewStage{results: copied}
}

func (s *scriptedSpecReviewStage) Name() string { return "spec-review" }

func (s *scriptedSpecReviewStage) Run(_ context.Context, _ *stagepkg.Request) (*stagepkg.Result, error) {
	s.calls++
	if len(s.results) == 0 {
		return &stagepkg.Result{Decision: stagepkg.DecisionProceed}, nil
	}
	next := s.results[0]
	s.results = s.results[1:]
	return next, nil
}

func containsLabel(labels []string, target string) bool {
	for _, label := range labels {
		if label == target {
			return true
		}
	}
	return false
}

func buildPostBeadLoopSpecLoop(t *testing.T, specID string, acceptStage stagepkg.Stage, specReviewStage stagepkg.Stage, remediationRunner remediationRunner, taskTrackerAdapter trackertypes.TaskTracker) (*SpecLoop, *fakePresentStage, *stagepresent.SummaryContext) {
	t.Helper()
	if acceptStage == nil {
		t.Fatalf("accept stage is required")
	}
	if taskTrackerAdapter == nil {
		taskTrackerAdapter = newFakeTaskTrackerAdapter()
	}

	presentStage := newFakePresentStage()
	summaryCtx := &stagepresent.SummaryContext{}
	git := newFakeGitAdapter(t)
	cfg := &config.Config{Paths: config.PathsConfig{GromitDir: ".gromit"}}
	adapters := adapter.AdapterSet{
		Git:         git,
		LLM:         newFakeLLMAdapter(),
		TaskTracker: taskTrackerAdapter,
		Presenter:   newFakePresenterAdapter(t),
	}
	opts := []SpecLoopOption{
		WithPlanStage(newFakePlanStage(specID)),
		WithPresentStage(presentStage, summaryCtx),
		WithDecomposeStage(newFakeDecomposeStage(specID)),
		WithBeadLoop(newFakeBeadRunner()),
		WithAcceptStage(acceptStage),
	}
	if specReviewStage != nil {
		opts = append(opts, WithSpecReviewStage(specReviewStage))
	}
	if remediationRunner != nil {
		opts = append(opts, WithRemediationRunner(remediationRunner))
	}

	loopInstance, err := NewSpecLoop(adapters, cfg, noopDependencyGate{}, opts...)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}
	return loopInstance, presentStage, summaryCtx
}
