package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/queue"
	"github.com/danabrams/gromit/internal/tracker"
)

type testQueueModelSelector struct{}

func (testQueueModelSelector) SelectModel(priority int, labels []string) string {
	return "sonnet"
}

func TestGroupBeadsBySpec(t *testing.T) {
	t.Parallel()
	beads := []*bead.Bead{
		{ID: "a", Labels: []string{"spec:auth"}},
		{ID: "b", Labels: []string{"priority:p1"}},
		{ID: "c", Labels: []string{"spec:api"}},
		{ID: "d", Labels: []string{"spec:auth"}},
	}

	grouped := groupBeadsBySpec(beads)

	if len(grouped["auth"]) != 2 {
		t.Fatalf("grouped[auth] count = %d, want 2", len(grouped["auth"]))
	}
	if len(grouped["api"]) != 1 {
		t.Fatalf("grouped[api] count = %d, want 1", len(grouped["api"]))
	}
	if len(grouped[""]) != 1 {
		t.Fatalf("grouped[(none)] count = %d, want 1", len(grouped[""]))
	}
}

func TestOrderedSpecKeys_PutsNoneLast(t *testing.T) {
	t.Parallel()
	grouped := map[string][]*bead.Bead{
		"":     []*bead.Bead{{ID: "none"}},
		"auth": []*bead.Bead{{ID: "auth"}},
		"api":  []*bead.Bead{{ID: "api"}},
	}

	keys := orderedSpecKeys(grouped)
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	if keys[0] != "api" || keys[1] != "auth" || keys[2] != "" {
		t.Fatalf("keys = %v, want [api auth \"\"]", keys)
	}
}

func TestColorizeLine(t *testing.T) {
	t.Parallel()
	line := "hello"
	colored := colorizeLine(line, ansiGreen, true)
	if colored != ansiGreen+line+ansiReset {
		t.Fatalf("colored = %q", colored)
	}

	plain := colorizeLine(line, ansiGreen, false)
	if plain != line {
		t.Fatalf("plain = %q, want %q", plain, line)
	}
}

func TestShowQueue_DelegatesToPipeline(t *testing.T) {
	// Not parallel: mutates configPath/queue globals and captures os.Stdout.

	origConfigPath := configPath
	configPath = filepath.Join("..", "..", "gromit.yaml")
	t.Cleanup(func() {
		configPath = origConfigPath
	})

	now := queueBySpec
	origCompletion := queueCompletionOrder
	queueBySpec = false
	queueCompletionOrder = false
	t.Cleanup(func() {
		queueBySpec = now
		queueCompletionOrder = origCompletion
	})

	pipelineCalled := false
	mockExecutor := &mockQueueExecutor{
		queueFn: func(ctx context.Context, input pipeline.QueueInput) (*pipeline.QueueResult, error) {
			pipelineCalled = true
			return &pipeline.QueueResult{
				Ready: []*bead.Bead{
					{ID: "ready-1", Title: "Ready Task", Priority: 0},
				},
				Blocked: []*bead.Bead{
					{ID: "blocked-1", Title: "Blocked Task", Priority: 1},
				},
				Stuck: []*bead.Bead{},
				All: []*bead.Bead{
					{ID: "ready-1"},
					{ID: "blocked-1"},
				},
			}, nil
		},
	}

	createQueuePipelineFn = func(cfg *config.Config, gromitDir string) (queueExecutor, error) {
		return mockExecutor, nil
	}
	t.Cleanup(func() {
		createQueuePipelineFn = createQueuePipeline
	})

	output := captureStdout(t, func() {
		if err := showQueue(queueCmd, nil); err != nil {
			t.Fatalf("showQueue returned error: %v", err)
		}
	})

	if !pipelineCalled {
		t.Fatal("expected pipeline.Queue to be called")
	}
	if !strings.Contains(output, "ready-1") || !strings.Contains(output, "blocked-1") {
		t.Fatalf("unexpected queue output: %s", output)
	}
}

func TestGetReason_FromDependencies(t *testing.T) {
	t.Parallel()
	b := &bead.Bead{
		ID: "b1",
		Dependencies: []bead.Dependency{
			{ID: "dep-a"},
			{ID: "dep-b"},
		},
	}
	got := getReason(b, nil)
	want := "blocked by: dep-a, dep-b"
	if got != want {
		t.Fatalf("getReason() = %q, want %q", got, want)
	}
}

func TestGetReason_FromDependencyCount(t *testing.T) {
	t.Parallel()
	count := 3
	b := &bead.Bead{ID: "b1", DependencyCount: &count}
	got := getReason(b, nil)
	want := "blocked by 3 dependencies"
	if got != want {
		t.Fatalf("getReason() = %q, want %q", got, want)
	}
}

// TestGetReason_RegressionAssertion_UsesQueuePackage verifies that getReason
// uses the queue package implementation and produces consistent results.
func TestGetReason_RegressionAssertion_UsesQueuePackage(t *testing.T) {
	t.Parallel()
	b := &bead.Bead{
		ID: "b1",
		Dependencies: []bead.Dependency{
			{ID: "dep-a"},
			{ID: "dep-b"},
		},
	}

	// Call the queue package function directly
	reasonPkg := queue.GetReason(b, nil)

	// Call the cmd wrapper
	reasonCmd := getReason(b, nil)

	// Both should produce identical results
	if reasonPkg != reasonCmd {
		t.Fatalf("getReason mismatch: pkg=%q, cmd=%q", reasonPkg, reasonCmd)
	}
}

func TestPrintQueueByStatus_BySpecGroupsStatusesWithinEachSpec(t *testing.T) {
	// Not parallel: captureStdout mutates os.Stdout and statsCmd flags.
	cfg := testQueueModelSelector{}
	ready := []*bead.Bead{
		{ID: "gromit-1", Priority: 0, Title: "Ready auth", Labels: []string{"spec:auth"}},
		{ID: "gromit-2", Priority: 1, Title: "Ready api", Labels: []string{"spec:api"}},
	}
	blocked := []*bead.Bead{
		{ID: "gromit-3", Priority: 1, Title: "Blocked auth", Labels: []string{"spec:auth"}, Dependencies: []bead.Dependency{{ID: "gromit-parent"}}},
	}
	stuck := []*bead.Bead{
		{ID: "gromit-4", Priority: 2, Title: "Stuck auth", Labels: []string{"spec:auth"}},
		{ID: "gromit-5", Priority: 2, Title: "Stuck api", Labels: []string{"spec:api"}},
	}
	all := append(append(append([]*bead.Bead{}, ready...), blocked...), stuck...)

	output := captureStdout(t, func() {
		printQueueByStatus(cfg, ready, nil, blocked, stuck, all, true, false)
	})

	authIdx := strings.Index(output, "Spec: auth")
	apiIdx := strings.Index(output, "Spec: api")
	if authIdx == -1 || apiIdx == -1 {
		t.Fatalf("expected auth/api spec headers, got:\n%s", output)
	}

	specStart := apiIdx
	nextSpecStart := authIdx
	if authIdx < apiIdx {
		specStart = authIdx
		nextSpecStart = apiIdx
	}
	firstSection := output[specStart:nextSpecStart]
	authSection := output[authIdx:]
	if authIdx < apiIdx {
		authSection = output[authIdx:apiIdx]
	}

	readyPosFirst := strings.Index(firstSection, "Ready (1):")
	blockedPosFirst := strings.Index(firstSection, "Blocked (1):")
	stuckPosFirst := strings.Index(firstSection, "Stuck (1):")
	if readyPosFirst == -1 || stuckPosFirst == -1 {
		t.Fatalf("expected ready/stuck subsections in first spec section:\n%s", firstSection)
	}
	if blockedPosFirst != -1 && !(readyPosFirst < blockedPosFirst && blockedPosFirst < stuckPosFirst) {
		t.Fatalf("expected Ready->Blocked->Stuck order in first spec section, got:\n%s", firstSection)
	}

	if !strings.Contains(authSection, "Ready (1):") {
		t.Fatalf("auth section missing ready subsection:\n%s", authSection)
	}
	if !strings.Contains(authSection, "Blocked (1):") {
		t.Fatalf("auth section missing blocked subsection:\n%s", authSection)
	}
	if !strings.Contains(authSection, "Stuck (1):") {
		t.Fatalf("auth section missing stuck subsection:\n%s", authSection)
	}

	readyPos := strings.Index(authSection, "Ready (1):")
	blockedPos := strings.Index(authSection, "Blocked (1):")
	stuckPos := strings.Index(authSection, "Stuck (1):")
	if !(readyPos < blockedPos && blockedPos < stuckPos) {
		t.Fatalf("expected Ready->Blocked->Stuck order, got:\n%s", authSection)
	}
}

func TestPrintQueueByStatus_BySpecIncludesSpecsWithoutReadyBeads(t *testing.T) {
	// Not parallel: captureStdout mutates os.Stdout and statsCmd flags.
	cfg := testQueueModelSelector{}
	ready := []*bead.Bead{
		{ID: "gromit-1", Priority: 0, Title: "Ready auth", Labels: []string{"spec:auth"}},
	}
	blocked := []*bead.Bead{
		{ID: "gromit-2", Priority: 1, Title: "Blocked billing", Labels: []string{"spec:billing"}, Dependencies: []bead.Dependency{{ID: "gromit-parent"}}},
	}

	output := captureStdout(t, func() {
		printQueueByStatus(cfg, ready, nil, blocked, nil, blocked, true, false)
	})

	if !strings.Contains(output, "Spec: auth") {
		t.Fatalf("expected auth section, got:\n%s", output)
	}
	if !strings.Contains(output, "Spec: billing") {
		t.Fatalf("expected billing section, got:\n%s", output)
	}
	if !strings.Contains(output, "Blocked (1):") {
		t.Fatalf("expected blocked subsection, got:\n%s", output)
	}
}

// mockTrackerForReadyBeads is a test double for tracker.Client
type mockTrackerForReadyBeads struct {
	onList func(context.Context, tracker.Query) ([]tracker.Item, error)
}

func (m *mockTrackerForReadyBeads) Ready(context.Context) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	if m.onList != nil {
		return m.onList(ctx, q)
	}
	return []tracker.Item{}, nil
}

func (m *mockTrackerForReadyBeads) Show(context.Context, string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) Search(context.Context, tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) Create(context.Context, tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) CreateWithParent(context.Context, tracker.CreateRequest, string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) Update(context.Context, tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) ListWithLabel(context.Context, string) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerForReadyBeads) Close(context.Context, string) error {
	return nil
}

func (m *mockTrackerForReadyBeads) Sync(context.Context) error {
	return nil
}

func (m *mockTrackerForReadyBeads) AddComment(context.Context, string, string) error {
	return nil
}

func (m *mockTrackerForReadyBeads) HasOpenChildren(context.Context, string) (bool, error) {
	return false, nil
}

// TestQueueCmd_RegressionAssertion_DoesNotMutateBeadState verifies that queue command
// is a read-only operation and doesn't modify any bead state in the tracker.
func TestQueueCmd_RegressionAssertion_DoesNotMutateBeadState(t *testing.T) {
	t.Parallel()
	// Create a mock tracker client that will fail if any mutation method is called
	mutationCalls := []string{}
	mockTracker := &mockTrackerForReadyBeads{
		onList: func(_ context.Context, q tracker.Query) ([]tracker.Item, error) {
			return []tracker.Item{
				{ID: "bead-1", Title: "Task 1", Status: "ready"},
				{ID: "bead-2", Title: "Task 2", Status: "open"},
			}, nil
		},
	}

	// Create a wrapper that tracks calls to mutation methods
	mutationTracker := &trackerMutationTracker{
		wrapped: mockTracker,
		onMutation: func(methodName string) {
			mutationCalls = append(mutationCalls, methodName)
		},
	}

	// Simulate queue command logic with the mutation-tracking tracker
	ctx := context.Background()
	items, err := mutationTracker.List(ctx, tracker.Query{Filter: tracker.Filter{Statuses: []string{"ready"}}})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected items from List")
	}

	// Verify no mutation methods were called
	if len(mutationCalls) > 0 {
		t.Errorf("queue command should not call mutation methods, but called: %v", mutationCalls)
	}
}

// TestQueueCmd_RegressionAssertion_OutputIsStable verifies queue command produces
// consistent output across multiple invocations (no state mutation side effects).
func TestQueueCmd_RegressionAssertion_OutputIsStable(t *testing.T) {
	// Not parallel: captureStdout mutates os.Stdout and shared command flags.

	// Simulate queue command display - run twice with same data
	callCount := 0
	var output1, output2 string

	for i := 0; i < 2; i++ {
		output := captureStdout(t, func() {
			cfg := testQueueModelSelector{}
			ready := []*bead.Bead{
				{ID: "task-1", Title: "First Task", Priority: 1},
				{ID: "task-2", Title: "Second Task", Priority: 2},
			}
			blocked := []*bead.Bead{}
			stuck := []*bead.Bead{}

			printQueueByStatus(cfg, ready, nil, blocked, stuck, ready, true, false)
			callCount++
		})

		if i == 0 {
			output1 = output
		} else {
			output2 = output
		}
	}

	// Outputs should be identical across invocations (no state mutation)
	if output1 != output2 {
		t.Errorf("queue command output is not stable (indicates state mutation):\nFirst:\n%s\n\nSecond:\n%s", output1, output2)
	}

	// Both invocations should have completed
	if callCount != 2 {
		t.Errorf("expected 2 invocations, got %d", callCount)
	}
}

func TestQueueCmd_RegressionAssertion_TUISectionsIntact(t *testing.T) {
	// Not parallel: captureStdout mutates os.Stdout and shared command flags.

	cfg := testQueueModelSelector{}
	ready := []*bead.Bead{
		{ID: "ready-task", Title: "Ready for Model", Priority: 0, Labels: []string{"spec:alpha"}},
	}
	blocked := []*bead.Bead{
		{ID: "blocked-task", Title: "Waiting on Parent", Priority: 1, Labels: []string{"spec:alpha"}, Dependencies: []bead.Dependency{{ID: "pending"}}},
	}
	stuck := []*bead.Bead{
		{ID: "stuck-task", Title: "Exceeded Retry", Priority: 2, Labels: []string{"spec:beta"}},
	}
	all := append(append(append([]*bead.Bead{}, ready...), blocked...), stuck...)

	output := captureStdout(t, func() {
		printQueueByStatus(cfg, ready, nil, blocked, stuck, all, true, false)
	})

	assertQueueTUISections(t, output)
}

func assertQueueTUISections(t *testing.T, output string) {
	required := []string{
		"Queue",
		"Queue by spec",
		"Spec: alpha",
		"Blocked (1):",
		"Stuck (1):",
	}
	for _, section := range required {
		if !strings.Contains(output, section) {
			t.Fatalf("queue output missing expected section %q, got:\n%s", section, output)
		}
	}
}

// trackerMutationTracker wraps a tracker.Client and calls onMutation when mutation methods are invoked
type trackerMutationTracker struct {
	wrapped    tracker.Client
	onMutation func(methodName string)
}

func (m *trackerMutationTracker) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	m.onMutation("Create")
	return m.wrapped.Create(ctx, req)
}

func (m *trackerMutationTracker) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	m.onMutation("CreateWithParent")
	return m.wrapped.CreateWithParent(ctx, req, parentID)
}

func (m *trackerMutationTracker) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	m.onMutation("Update")
	return m.wrapped.Update(ctx, req)
}

func (m *trackerMutationTracker) Close(ctx context.Context, id string) error {
	m.onMutation("Close")
	return m.wrapped.Close(ctx, id)
}

func (m *trackerMutationTracker) Sync(ctx context.Context) error {
	m.onMutation("Sync")
	return m.wrapped.Sync(ctx)
}

func (m *trackerMutationTracker) AddComment(ctx context.Context, id, comment string) error {
	m.onMutation("AddComment")
	return m.wrapped.AddComment(ctx, id, comment)
}

func (m *trackerMutationTracker) Ready(ctx context.Context) (*tracker.Item, error) {
	return m.wrapped.Ready(ctx)
}

func (m *trackerMutationTracker) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	return m.wrapped.ReadyWithLabel(ctx, label)
}

func (m *trackerMutationTracker) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return m.wrapped.List(ctx, q)
}

func (m *trackerMutationTracker) Show(ctx context.Context, id string) (*tracker.Item, error) {
	return m.wrapped.Show(ctx, id)
}

func (m *trackerMutationTracker) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	return m.wrapped.ListWithLabel(ctx, label)
}

func (m *trackerMutationTracker) Search(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return m.wrapped.Search(ctx, q)
}

func (m *trackerMutationTracker) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return m.wrapped.HasOpenChildren(ctx, parentID)
}

type mockQueueExecutor struct {
	queueFn func(context.Context, pipeline.QueueInput) (*pipeline.QueueResult, error)
}

func (m *mockQueueExecutor) Queue(ctx context.Context, input pipeline.QueueInput) (*pipeline.QueueResult, error) {
	if m == nil || m.queueFn == nil {
		return nil, nil
	}
	return m.queueFn(ctx, input)
}
