package pipeline

import (
    "context"
    "encoding/json"
    "fmt"
    "reflect"
    "strings"
    "testing"
    "time"

    "github.com/danabrams/gromit/internal/review"
)

func TestApplyReviewFindings_CalledPathsProduceIdenticalTrackerAndBacklogCalls(t *testing.T) {
    t.Parallel()

    reviewResult := mixedReviewResult()
    ctx := context.Background()

    directTracker := newCapturingTrackerClient()
    directBacklog := newCapturingBacklogWriter()
    directDeps := &Deps{
        TrackerClient:    directTracker,
        BacklogWriter:    directBacklog,
        LearningsManager: newRecordingLearningsManager(),
    }
    directPipeline := New(directDeps, &Paths{GromitDir: t.TempDir()})

    directApply, err := directPipeline.ApplyReviewFindings(ctx, reviewResult)
    if err != nil {
        t.Fatalf("ApplyReviewFindings direct path error: %v", err)
    }

    encoded, err := json.Marshal(reviewResult)
    if err != nil {
        t.Fatalf("marshal review result: %v", err)
    }

    nonInteractiveTracker := newCapturingTrackerClient()
    nonInteractiveBacklog := newCapturingBacklogWriter()
    invoker := &reviewAcceptanceMockReviewInvoker{
        runFunc: func(prompt string, model string, timeout time.Duration) (*LLMRunResult, error) {
            return &LLMRunResult{Success: true, Output: string(encoded)}, nil
        },
    }
    renderer := &reviewAcceptanceMockReviewRenderer{
        renderThoroughReviewFunc: func(input *ThoroughReviewPromptInput) (string, error) {
            return "# Review Prompt", nil
        },
    }
    nonInteractiveDeps := &Deps{
        ReviewRenderer:   renderer,
        ReviewInvoker:    invoker,
        TrackerClient:    nonInteractiveTracker,
        BacklogWriter:    nonInteractiveBacklog,
        LearningsManager: newRecordingLearningsManager(),
        LogWriter:        &reviewAcceptanceMockLogWriter{},
        StateManager:     &reviewAcceptanceMockStateManager{},
    }
    nonInteractivePipeline := New(nonInteractiveDeps, &Paths{GromitDir: t.TempDir()})

    nonInteractiveResult, err := nonInteractivePipeline.ReviewNonInteractive(ctx, ReviewInput{
        FromCommit: "abc123",
        Diff:       "diff",
        Model:      "sonnet",
        Timeout:    60,
    })
    if err != nil {
        t.Fatalf("ReviewNonInteractive error: %v", err)
    }

    compareTrackerCalls(t, directTracker.calls, nonInteractiveTracker.calls)
    compareBacklogEntries(t, directBacklog.entries, nonInteractiveBacklog.entries)

    if nonInteractiveResult.Apply == nil {
        t.Fatalf("non-interactive result Apply field is nil")
    }
    if !reflect.DeepEqual(directApply.CreatedBeadIDs, nonInteractiveResult.Apply.CreatedBeadIDs) {
        t.Fatalf("bead IDs differ between paths: %v vs %v", directApply.CreatedBeadIDs, nonInteractiveResult.Apply.CreatedBeadIDs)
    }
}

func TestReviewApplyResult_MixedResultCountsAndIDs(t *testing.T) {
	t.Parallel()

    reviewResult := mixedReviewResult()
    ctx := context.Background()

    tracker := newCapturingTrackerClient()
    backlog := newCapturingBacklogWriter()
    deps := &Deps{
        TrackerClient:    tracker,
        BacklogWriter:    backlog,
        LearningsManager: newRecordingLearningsManager(),
    }
    pipeline := New(deps, &Paths{GromitDir: t.TempDir()})

    applyResult, err := pipeline.ApplyReviewFindings(ctx, reviewResult)
    if err != nil {
        t.Fatalf("ApplyReviewFindings error: %v", err)
    }

    if len(applyResult.CreatedBeadIDs) != len(reviewResult.BeadsToCreate) {
        t.Fatalf("expected %d bead IDs, got %d", len(reviewResult.BeadsToCreate), len(applyResult.CreatedBeadIDs))
    }
	if applyResult.CreatedBacklogCount != len(reviewResult.BacklogItems) {
		t.Fatalf("expected backlog count %d, got %d", len(reviewResult.BacklogItems), applyResult.CreatedBacklogCount)
	}
	if applyResult.LearningsSaved != len(reviewResult.Learnings) {
		t.Fatalf("expected learnings saved %d, got %d", len(reviewResult.Learnings), applyResult.LearningsSaved)
	}
}

func TestApplyReviewFindings_PreservesBeadLabels(t *testing.T) {
	t.Parallel()

	reviewResult := &review.ReviewResult{
		BeadsToCreate: []review.BeadProposal{
			{
				Title:    "Document API",
				Labels:   []string{"bug", "from-review", "urgent"},
				Priority: 2,
			},
		},
	}
	ctx := context.Background()

	tracker := newCapturingTrackerClient()
	backlog := newCapturingBacklogWriter()
	deps := &Deps{
		TrackerClient:    tracker,
		BacklogWriter:    backlog,
		LearningsManager: newRecordingLearningsManager(),
	}
	pipeline := New(deps, &Paths{GromitDir: t.TempDir()})

	if _, err := pipeline.ApplyReviewFindings(ctx, reviewResult); err != nil {
		t.Fatalf("ApplyReviewFindings error: %v", err)
	}

	if len(tracker.calls) != 1 {
		t.Fatalf("expected tracker to be called once, got %d", len(tracker.calls))
	}

	wantLabels := []string{"from-review", "bug", "urgent"}
	if !reflect.DeepEqual(tracker.calls[0].labels, wantLabels) {
		t.Fatalf("unexpected labels, want %v, got %v", wantLabels, tracker.calls[0].labels)
	}
}

func TestApplyReviewFindings_PropagatesTrackerError(t *testing.T) {
	t.Parallel()

	reviewResult := &review.ReviewResult{
		BeadsToCreate: []review.BeadProposal{
			{Title: "Fix panic", Labels: []string{"bug"}, Priority: 1},
		},
	}
	ctx := context.Background()

	tracker := newFailingTrackerClient(fmt.Errorf("boom"))
	backlog := newCapturingBacklogWriter()
	deps := &Deps{
		TrackerClient:    tracker,
		BacklogWriter:    backlog,
		LearningsManager: newRecordingLearningsManager(),
	}
	pipeline := New(deps, &Paths{GromitDir: t.TempDir()})

		if _, err := pipeline.ApplyReviewFindings(ctx, reviewResult); err == nil {
			t.Fatalf("expected error from tracker create, got nil")
		} else if !strings.Contains(err.Error(), "creating review bead \"Fix panic\"") {
			t.Fatalf("unexpected error message: %v", err)
		}
}

func TestBuildReviewBacklogEntry_AssemblesDescriptionAndLabels(t *testing.T) {
	t.Parallel()

	item := review.BacklogItem{
		Title:       "Refactor cache",
		Description: "Cache is convoluted",
		Reason:      "performance",
	}

	entry := buildReviewBacklogEntry(item)

	wantDescription := "Cache is convoluted\n\nReason for backlog: performance"
	if entry.Description != wantDescription {
		t.Fatalf("unexpected description, want %q, got %q", wantDescription, entry.Description)
	}

	wantLabels := review.BuildBacklogLabels()
	if !reflect.DeepEqual(entry.Labels, wantLabels) {
		t.Fatalf("unexpected labels, want %v, got %v", wantLabels, entry.Labels)
	}

	wantOutputs := []string{"Refactor cache"}
	if !reflect.DeepEqual(entry.ExpectedOutputs, wantOutputs) {
		t.Fatalf("unexpected expected outputs, want %v, got %v", wantOutputs, entry.ExpectedOutputs)
	}

	if entry.Priority != backlogPriorityDefault {
		t.Fatalf("unexpected priority, want %d, got %d", backlogPriorityDefault, entry.Priority)
	}
}

func TestApplyBacklogItems_ErrorPropagates(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	count, err := applyBacklogItems(ctx, []review.BacklogItem{{Title: "Refactor cache"}}, newFailingBacklogWriter(fmt.Errorf("boom")))
	if err == nil {
		t.Fatalf("expected error from applyBacklogItems, got nil")
	}
	if count != 0 {
		t.Fatalf("expected count 0 on failure, got %d", count)
	}
	if !strings.Contains(err.Error(), "creating backlog item \"Refactor cache\"") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPersistLearnings_ErrorPropagates(t *testing.T) {
	t.Parallel()

	count, err := persistLearnings([]string{"Log errors"}, newFailingLearningsManager(fmt.Errorf("boom")))
	if err == nil {
		t.Fatalf("expected error from persistLearnings, got nil")
	}
	if count != 0 {
		t.Fatalf("expected count 0 on failure, got %d", count)
	}
	if !strings.Contains(err.Error(), "persisting learning") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func mixedReviewResult() *review.ReviewResult {
    return &review.ReviewResult{
        Passed: true,
        FixesApplied: []string{"fix formatting"},
        BeadsToCreate: []review.BeadProposal{
            {
                Title: "Add logging",
                Priority: 1,
                Labels: []string{"observability"},
            },
        },
        BacklogItems: []review.BacklogItem{
            {
                Title: "Refactor cache",
                Reason: "performance",
            },
        },
        Learnings: []string{"Log errors"},
    }
}

type trackerCall struct {
    title    string
    priority int
    labels   []string
    outputs  []string
}

type capturingTrackerClient struct {
    calls []trackerCall
}

func newCapturingTrackerClient() *capturingTrackerClient {
    return &capturingTrackerClient{}
}

func (c *capturingTrackerClient) Ready(ctx context.Context) (*BeadInfo, error) { return nil, nil }
func (c *capturingTrackerClient) Show(ctx context.Context, id string) (*BeadInfo, error) { return nil, nil }
func (c *capturingTrackerClient) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
    c.calls = append(c.calls, trackerCall{title: title, priority: priority, labels: append([]string(nil), labels...), outputs: append([]string(nil), outputs...)})
    return &BeadInfo{ID: fmt.Sprintf("bead-%d", len(c.calls))}, nil
}
func (c *capturingTrackerClient) CreateWithDepsAndDescription(ctx context.Context, title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
    return nil, fmt.Errorf("not implemented")
}
func (c *capturingTrackerClient) Close(ctx context.Context, id string) error { return nil }
func (c *capturingTrackerClient) ListWithLabel(ctx context.Context, label string) ([]string, error) { return []string{}, nil }

func compareTrackerCalls(t *testing.T, want, got []trackerCall) {
    if len(want) != len(got) {
        t.Fatalf("tracker call count mismatch: want %d, got %d", len(want), len(got))
    }
    for i := range want {
        if want[i].title != got[i].title || want[i].priority != got[i].priority || !reflect.DeepEqual(want[i].labels, got[i].labels) || !reflect.DeepEqual(want[i].outputs, got[i].outputs) {
            t.Fatalf("tracker call %d differs: want %+v, got %+v", i, want[i], got[i])
        }
    }
}

type capturingBacklogWriter struct {
	entries []*BacklogEntry
}

func newCapturingBacklogWriter() *capturingBacklogWriter {
    return &capturingBacklogWriter{}
}

func (c *capturingBacklogWriter) Add(ctx context.Context, entry *BacklogEntry) error {
    copy := *entry
    copy.Labels = append([]string(nil), entry.Labels...)
    copy.ExpectedOutputs = append([]string(nil), entry.ExpectedOutputs...)
    c.entries = append(c.entries, &copy)
    return nil
}

func (c *capturingBacklogWriter) Update(id string, fn func(*Idea)) error { return nil }

func compareBacklogEntries(t *testing.T, want, got []*BacklogEntry) {
	if len(want) != len(got) {
		t.Fatalf("backlog entry count mismatch: want %d, got %d", len(want), len(got))
	}
	for i := range want {
		if !reflect.DeepEqual(want[i], got[i]) {
			t.Fatalf("backlog entry %d differs: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

type failingBacklogWriter struct {
	err error
}

func newFailingBacklogWriter(err error) *failingBacklogWriter {
	return &failingBacklogWriter{err: err}
}

func (c *failingBacklogWriter) Add(ctx context.Context, entry *BacklogEntry) error {
	return c.err
}

func (c *failingBacklogWriter) Update(id string, fn func(*Idea)) error { return nil }

type failingTrackerClient struct {
    err error
}

func newFailingTrackerClient(err error) *failingTrackerClient {
    return &failingTrackerClient{err: err}
}

func (c *failingTrackerClient) Ready(ctx context.Context) (*BeadInfo, error) { return nil, nil }
func (c *failingTrackerClient) Show(ctx context.Context, id string) (*BeadInfo, error) { return nil, nil }
func (c *failingTrackerClient) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
    return nil, c.err
}
func (c *failingTrackerClient) CreateWithDepsAndDescription(ctx context.Context, title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
    return nil, fmt.Errorf("not implemented")
}
func (c *failingTrackerClient) Close(ctx context.Context, id string) error { return nil }
func (c *failingTrackerClient) ListWithLabel(ctx context.Context, label string) ([]string, error) { return []string{}, nil }

type recordingLearningsManager struct {
	saved []string
}

func newRecordingLearningsManager() *recordingLearningsManager {
    return &recordingLearningsManager{}
}

func (r *recordingLearningsManager) Add(content string) error {
	r.saved = append(r.saved, content)
	return nil
}

type failingLearningsManager struct {
	err error
}

func newFailingLearningsManager(err error) *failingLearningsManager {
	return &failingLearningsManager{err: err}
}

func (f *failingLearningsManager) Add(content string) error {
	return f.err
}
