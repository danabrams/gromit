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

func TestTrackerBeadClientListWithLabelConvertsItems(t *testing.T) {
	t.Parallel()

	items := []tracker.Item{
		{
			ID:          "bead-1",
			Title:       "Bead 1",
			Status:      "open",
			Description: "Description 1",
			Metadata: map[string]string{
				"priority": "2",
				"labels":   `["spec:test"]`,
			},
		},
		{
			ID:          "bead-2",
			Title:       "Bead 2",
			Status:      "open",
			Description: "Description 2",
			Metadata: map[string]string{
				"priority": "3",
			},
		},
	}

	client := &stubTrackerClient{
		listWithLabelFn: func(ctx context.Context, label string) ([]tracker.Item, error) {
			return items, nil
		},
	}

	beads := newTrackerBeadClient(client)
	result, err := beads.ListWithLabel(context.Background(), "test-label")
	if err != nil {
		t.Fatalf("ListWithLabel returned error: %v", err)
	}
	if result == nil {
		t.Fatal("ListWithLabel returned nil")
	}
	if len(result) != 2 {
		t.Fatalf("ListWithLabel returned %d beads, want 2", len(result))
	}
	if result[0].ID != "bead-1" {
		t.Fatalf("first bead ID = %s, want bead-1", result[0].ID)
	}
	if result[1].ID != "bead-2" {
		t.Fatalf("second bead ID = %s, want bead-2", result[1].ID)
	}
}

func TestTrackerBeadClientReadyExcludingFiltersEpicsAndExcludedIDs(t *testing.T) {
	t.Parallel()

	items := []tracker.Item{
		{
			ID:     "epic-1",
			Title:  "Epic",
			Status: "open",
			Metadata: map[string]string{
				"type": "epic",
			},
		},
		{
			ID:     "excluded-1",
			Title:  "Excluded Bead",
			Status: "open",
			Metadata: map[string]string{
				"priority": "1",
			},
		},
		{
			ID:     "bead-1",
			Title:  "Valid Bead",
			Status: "open",
			Metadata: map[string]string{
				"priority": "1",
			},
		},
	}

	client := &stubTrackerClient{
		listFn: func(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
			return items, nil
		},
	}

	beads := newTrackerBeadClient(client)
	bead, err := beads.ReadyExcluding(context.Background(), map[string]bool{"excluded-1": true})
	if err != nil {
		t.Fatalf("ReadyExcluding returned error: %v", err)
	}
	if bead == nil {
		t.Fatal("ReadyExcluding returned nil bead")
	}
	if bead.ID != "bead-1" {
		t.Fatalf("bead ID = %s, want bead-1", bead.ID)
	}
}

type stubTrackerClient struct {
	readyFn         func(ctx context.Context) (*tracker.Item, error)
	listFn          func(ctx context.Context, q tracker.Query) ([]tracker.Item, error)
	listWithLabelFn func(ctx context.Context, label string) ([]tracker.Item, error)
}

func (s *stubTrackerClient) Ready(ctx context.Context) (*tracker.Item, error) {
	if s.readyFn != nil {
		return s.readyFn(ctx)
	}
	return nil, nil
}
func (s *stubTrackerClient) List(ctx context.Context, q tracker.Query) ([]tracker.Item, error) {
	if s.listFn != nil {
		return s.listFn(ctx, q)
	}
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
	if s.listWithLabelFn != nil {
		return s.listWithLabelFn(ctx, label)
	}
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
