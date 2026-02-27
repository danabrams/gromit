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

func TestTrackerBeadClientAddCommentPassesThrough(t *testing.T) {
	t.Parallel()

	var capturedID string
	var capturedComment string
	client := &stubTrackerClient{
		addCommentFn: func(ctx context.Context, id, comment string) error {
			capturedID = id
			capturedComment = comment
			return nil
		},
	}

	beads := newTrackerBeadClient(client)
	err := beads.AddComment(context.Background(), "bead-1", "Test comment")
	if err != nil {
		t.Fatalf("AddComment returned error: %v", err)
	}
	if capturedID != "bead-1" {
		t.Fatalf("captured ID = %s, want bead-1", capturedID)
	}
	if capturedComment != "Test comment" {
		t.Fatalf("captured comment = %s, want Test comment", capturedComment)
	}
}

func TestTrackerBeadClientClosePassesThrough(t *testing.T) {
	t.Parallel()

	var capturedID string
	client := &stubTrackerClient{
		closeFn: func(ctx context.Context, id string) error {
			capturedID = id
			return nil
		},
	}

	beads := newTrackerBeadClient(client)
	err := beads.Close(context.Background(), "bead-1")
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if capturedID != "bead-1" {
		t.Fatalf("captured ID = %s, want bead-1", capturedID)
	}
}

func TestTrackerBeadClientSyncPassesThrough(t *testing.T) {
	t.Parallel()

	syncCalled := false
	client := &stubTrackerClient{
		syncFn: func(ctx context.Context) error {
			syncCalled = true
			return nil
		},
	}

	beads := newTrackerBeadClient(client)
	err := beads.Sync(context.Background())
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if !syncCalled {
		t.Fatal("Sync was not called on the underlying client")
	}
}

func TestTrackerBeadClientCreateEncodesMetadata(t *testing.T) {
	t.Parallel()

	createdItem := &tracker.Item{
		ID:    "new-bead",
		Title: "New Bead",
		Status: "open",
		Metadata: map[string]string{
			"priority": "2",
		},
	}

	var capturedReq tracker.CreateRequest
	client := &stubTrackerClient{
		createFn: func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
			capturedReq = req
			return createdItem, nil
		},
	}

	beads := newTrackerBeadClient(client)
	result, err := beads.Create(context.Background(), "Test Title", 2, []string{"label1"}, []string{"output1"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Create returned nil")
	}
	if result.ID != "new-bead" {
		t.Fatalf("created bead ID = %s, want new-bead", result.ID)
	}

	// Verify metadata encoding
	if capturedReq.Title != "Test Title" {
		t.Fatalf("captured request title = %s, want Test Title", capturedReq.Title)
	}
	if capturedReq.Metadata["priority"] != "2" {
		t.Fatalf("captured priority = %s, want 2", capturedReq.Metadata["priority"])
	}
	if capturedReq.Metadata["labels"] != `["label1"]` {
		t.Fatalf("captured labels = %s, want [\"label1\"]", capturedReq.Metadata["labels"])
	}
	if capturedReq.Metadata["expected_outputs"] != `["output1"]` {
		t.Fatalf("captured expected_outputs = %s, want [\"output1\"]", capturedReq.Metadata["expected_outputs"])
	}
}

func TestTrackerBeadClientCreateWithParentEncodesMetadata(t *testing.T) {
	t.Parallel()

	createdItem := &tracker.Item{
		ID:    "new-child",
		Title: "New Child",
		Status: "open",
		Metadata: map[string]string{
			"priority": "2",
			"parent":   "parent-id",
		},
	}

	var capturedReq tracker.CreateRequest
	var capturedParentID string
	client := &stubTrackerClient{
		createWithParentFn: func(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
			capturedReq = req
			capturedParentID = parentID
			return createdItem, nil
		},
	}

	beads := newTrackerBeadClient(client)
	result, err := beads.CreateWithParent(context.Background(), "Test Title", 2, []string{"label1"}, []string{"output1"}, "parent-id")
	if err != nil {
		t.Fatalf("CreateWithParent returned error: %v", err)
	}
	if result == nil {
		t.Fatal("CreateWithParent returned nil")
	}
	if result.ID != "new-child" {
		t.Fatalf("created bead ID = %s, want new-child", result.ID)
	}
	if capturedParentID != "parent-id" {
		t.Fatalf("captured parent ID = %s, want parent-id", capturedParentID)
	}

	// Verify metadata encoding
	if capturedReq.Title != "Test Title" {
		t.Fatalf("captured request title = %s, want Test Title", capturedReq.Title)
	}
	if capturedReq.Metadata["priority"] != "2" {
		t.Fatalf("captured priority = %s, want 2", capturedReq.Metadata["priority"])
	}
	if capturedReq.Metadata["labels"] != `["label1"]` {
		t.Fatalf("captured labels = %s, want [\"label1\"]", capturedReq.Metadata["labels"])
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
	readyFn              func(ctx context.Context) (*tracker.Item, error)
	listFn               func(ctx context.Context, q tracker.Query) ([]tracker.Item, error)
	listWithLabelFn      func(ctx context.Context, label string) ([]tracker.Item, error)
	createFn             func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error)
	createWithParentFn   func(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error)
	addCommentFn         func(ctx context.Context, id, comment string) error
	closeFn              func(ctx context.Context, id string) error
	syncFn               func(ctx context.Context) error
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
	if s.createFn != nil {
		return s.createFn(ctx, req)
	}
	return nil, nil
}
func (s *stubTrackerClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	if s.createWithParentFn != nil {
		return s.createWithParentFn(ctx, req, parentID)
	}
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
	if s.closeFn != nil {
		return s.closeFn(ctx, id)
	}
	return nil
}
func (s *stubTrackerClient) Sync(ctx context.Context) error {
	if s.syncFn != nil {
		return s.syncFn(ctx)
	}
	return nil
}
func (s *stubTrackerClient) AddComment(ctx context.Context, id, comment string) error {
	if s.addCommentFn != nil {
		return s.addCommentFn(ctx, id, comment)
	}
	return nil
}
func (s *stubTrackerClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}
