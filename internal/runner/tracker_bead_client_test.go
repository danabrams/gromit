package runner

import (
	"context"
	"testing"

	"github.com/danabrams/gromit/internal/tracker"
)

func TestTrackerBeadClientReadyReturnsConvertedBead(t *testing.T) {
	t.Parallel()

	trackerItem := &tracker.Item{
		ID:          "bead-1",
		Title:       "Title",
		Description: "Description",
		Status:      "open",
		Metadata: map[string]string{
			"priority": "2",
			"status":   "open",
			"labels":   `["spec:test"]`,
		},
	}
	client := &stubTrackerClient{
		readyFn: func(ctx context.Context) (*tracker.Item, error) {
			return trackerItem, nil
		},
	}

	beads := newTrackerBeadClient(client)
	bead, err := beads.Ready(context.Background())
	if err != nil {
		t.Fatalf("Ready returned error: %v", err)
	}
	if bead == nil {
		t.Fatal("Ready returned nil bead")
	}
	if bead.ID != trackerItem.ID {
		t.Fatalf("bead ID = %s, want %s", bead.ID, trackerItem.ID)
	}
	if bead.Status != trackerItem.Status {
		t.Fatalf("bead status = %s, want %s", bead.Status, trackerItem.Status)
	}
}

type stubTrackerClient struct {
	readyFn func(ctx context.Context) (*tracker.Item, error)
}

func (s *stubTrackerClient) Ready(ctx context.Context) (*tracker.Item, error) {
	if s.readyFn != nil {
		return s.readyFn(ctx)
	}
	return nil, nil
}
func (s *stubTrackerClient) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) Show(ctx context.Context, id string) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) Search(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	return nil, nil
}
func (s *stubTrackerClient) Close(ctx context.Context, id string) error {
	return nil
}
func (s *stubTrackerClient) Sync(ctx context.Context) error {
	return nil
}
func (s *stubTrackerClient) AddComment(ctx context.Context, id, comment string) error {
	return nil
}
func (s *stubTrackerClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}
