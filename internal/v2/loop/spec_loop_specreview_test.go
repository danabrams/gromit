package loop

import (
    "context"
    "reflect"
    "testing"

    "github.com/danabrams/gromit/internal/v2/adapter"
    stagepkg "github.com/danabrams/gromit/internal/v2/stage"
    "github.com/danabrams/gromit/internal/v2/stage/specreview"
    "github.com/danabrams/gromit/internal/v2/trackertypes"
)

func TestSpecLoopEnsureAcceptanceAndReviewCreatesFromReviewBeads(t *testing.T) {
    t.Parallel()

    ctx := context.Background()
    specID := "spec-review-loop"

    tracker := newRecordingTaskTracker()
    acceptStage := newFakeAcceptStage()

    reviewFindings := []stagepkg.Finding{
        {
            Severity:    stagepkg.FindingSeverityWarning,
            Category:    stagepkg.FindingCategoryQuality,
            Scope:       stagepkg.FindingScopeSpec,
            Description: "spec scoped warning",
        },
        {
            Severity:    stagepkg.FindingSeveritySuggestion,
            Category:    stagepkg.FindingCategoryQuality,
            Scope:       stagepkg.FindingScopeGeneral,
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
        adapters: adapter.AdapterSet{TaskTracker: tracker},
        acceptStage: acceptStage,
        specReviewStage: specReviewStage,
    }

    req := stagepkg.Request{Bead: stagepkg.BeadInfo{ID: specID}, Worktree: "worktree"}
    if _, err := s.ensureAcceptanceAndReview(ctx, &req, specID); err != nil {
        t.Fatalf("ensure acceptance and review: %v", err)
    }

    if got, want := len(tracker.created), len(reviewFindings); got != want {
        t.Fatalf("created beads = %d, want %d", got, want)
    }

    for idx, created := range tracker.created {
        finding := reviewFindings[idx]
        wantLabels := []string{"from-review"}
        if finding.Scope == stagepkg.FindingScopeSpec {
            wantLabels = append(wantLabels, "spec:"+specID)
        }
        if !reflect.DeepEqual(created.Labels, wantLabels) {
            t.Fatalf("labels for finding[%d] = %v, want %v", idx, created.Labels, wantLabels)
        }
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
