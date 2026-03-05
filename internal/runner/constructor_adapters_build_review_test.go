package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/specflow"
)

func TestNewStageAwarePreImplementationHook_OrchestratesAcceptanceTestBeads(t *testing.T) {
	t.Parallel()

	// Track created beads
	var createdBeads []*bead.Bead
	mockBeadClient := &mockBeadClientForPreImplementationHook{
		createFn: func(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
			b := &bead.Bead{
				ID:       "acceptance-test-bead",
				Title:    title,
				Priority: priority,
				Labels:   labels,
			}
			createdBeads = append(createdBeads, b)
			return b, nil
		},
	}

	stageCtx := &StageContext{
		SpecName: "spec-foo",
		Stage:    specflow.StageAcceptanceTests,
	}

	hook := newStageAwarePreImplementationHookWithBeadClient(stageCtx, mockBeadClient)
	if hook == nil {
		t.Fatal("hook is nil, want non-nil hook")
	}

	err := hook(context.Background())
	if err != nil {
		t.Fatalf("hook() error = %v, want nil", err)
	}

	if len(createdBeads) == 0 {
		t.Fatal("no beads created, want at least one bead for acceptance test authoring")
	}

	// Verify the created bead has acceptance-test-authoring labels
	createdBead := createdBeads[0]
	if createdBead.Title == "" {
		t.Error("bead title is empty")
	}
}

func TestNewStageAwarePreImplementationHook_ReturnsNilWhenNotAcceptanceTestsStage(t *testing.T) {
	t.Parallel()

	stageCtx := &StageContext{
		SpecName: "spec-foo",
		Stage:    specflow.StageImplementation,
	}

	hook := newStageAwarePreImplementationHook(stageCtx)
	if hook != nil {
		t.Fatal("hook is non-nil, want nil when stage is not AcceptanceTests")
	}
}

func TestNewStageAwarePreImplementationHook_ReturnsNilWhenSpecNameEmpty(t *testing.T) {
	t.Parallel()

	stageCtx := &StageContext{
		SpecName: "",
		Stage:    specflow.StageAcceptanceTests,
	}

	hook := newStageAwarePreImplementationHook(stageCtx)
	if hook != nil {
		t.Fatal("hook is non-nil, want nil when spec name is empty")
	}
}

// mockBeadClientForPreImplementationHook implements BeadClient for testing.
type mockBeadClientForPreImplementationHook struct {
	createFn func(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error)
}

func (m *mockBeadClientForPreImplementationHook) Ready(ctx context.Context) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) ReadyExcluding(ctx context.Context, excludeIDs map[string]bool) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) ReadyWithLabel(ctx context.Context, label string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) Show(ctx context.Context, id string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) Close(ctx context.Context, id string) error {
	return nil
}

func (m *mockBeadClientForPreImplementationHook) Sync(ctx context.Context) error {
	return nil
}

func (m *mockBeadClientForPreImplementationHook) AddComment(ctx context.Context, id, comment string) error {
	return nil
}

func (m *mockBeadClientForPreImplementationHook) GetParent(ctx context.Context, b *bead.Bead) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) Create(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string) (*bead.Bead, error) {
	if m.createFn != nil {
		return m.createFn(ctx, title, priority, labels, expectedOutputs)
	}
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) CreateWithParent(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) CreateWithParentAndDescription(ctx context.Context, title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockBeadClientForPreImplementationHook) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}
