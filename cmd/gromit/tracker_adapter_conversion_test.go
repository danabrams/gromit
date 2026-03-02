package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/tracker"
)

// TestTrackerClientAdapter_ConvertsTrackerItemToBeadInfo verifies trackerClientAdapter
// correctly converts tracker.Item with metadata to pipeline.BeadInfo.
func TestTrackerClientAdapter_ConvertsTrackerItemToBeadInfo(t *testing.T) {
	t.Parallel()

	// Create a mock tracker.Client that returns items with metadata
	mockClient := &mockTrackerClient{
		readyItem: &tracker.Item{
			ID:          "test-123",
			Title:       "Test Bead",
			Description: "Test description",
			Status:      tracker.StatusOpen,
			Metadata: map[string]string{
				"priority": "2",
				"labels":   `["spec:test","important"]`,
			},
		},
	}

	adapter := &trackerClientAdapter{Client: mockClient}

	// Call Ready with context
	ctx := context.Background()
	result, err := adapter.Ready(ctx)

	if err != nil {
		t.Fatalf("Ready() error: %v", err)
	}

	if result == nil {
		t.Fatal("Ready() returned nil result")
	}

	// Verify the conversion worked correctly
	if result.ID != "test-123" {
		t.Errorf("ID mismatch: got %q, want %q", result.ID, "test-123")
	}

	if result.Title != "Test Bead" {
		t.Errorf("Title mismatch: got %q, want %q", result.Title, "Test Bead")
	}

	if result.Priority != 2 {
		t.Errorf("Priority mismatch: got %d, want %d", result.Priority, 2)
	}

	// Verify it's using pipeline.BeadInfo type
	_ = pipeline.BeadInfo{} // Use pipeline package to avoid unused import
}

// mockTrackerClient implements tracker.Client for testing
type mockTrackerClient struct {
	readyItem  *tracker.Item
	showItems  map[string]*tracker.Item
	createdItems []*tracker.Item
}

func (m *mockTrackerClient) Ready(ctx context.Context) (*tracker.Item, error) {
	return m.readyItem, nil
}

func (m *mockTrackerClient) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClient) List(ctx context.Context, query tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClient) Show(ctx context.Context, id string) (*tracker.Item, error) {
	if m.showItems != nil {
		if item, ok := m.showItems[id]; ok {
			return item, nil
		}
	}
	return nil, nil
}

func (m *mockTrackerClient) Search(ctx context.Context, query tracker.Query) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClient) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	item := &tracker.Item{
		ID:       "created-item",
		Title:    req.Title,
		Metadata: req.Metadata,
	}
	m.createdItems = append(m.createdItems, item)
	return item, nil
}
func (m *mockTrackerClient) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	return m.Create(ctx, req)
}
func (m *mockTrackerClient) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	return nil, nil
}
func (m *mockTrackerClient) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	return nil, nil
}

func (m *mockTrackerClient) Close(ctx context.Context, id string) error {
	return nil
}

func (m *mockTrackerClient) Sync(ctx context.Context) error {
	return nil
}

func (m *mockTrackerClient) AddComment(ctx context.Context, id, comment string) error {
	return nil
}

func (m *mockTrackerClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	return false, nil
}

// TestTrackerClientAdapter_LabelsRoundtrip verifies that labels can be encoded and decoded correctly.
func TestTrackerClientAdapter_LabelsRoundtrip(t *testing.T) {
	t.Parallel()

	mockClient := &mockTrackerClient{}
	adapter := &trackerClientAdapter{Client: mockClient}

	ctx := context.Background()

	// Create a bead with labels
	inputLabels := []string{"spec:test", "important", "review"}
	created, err := adapter.Create(ctx, "Test Title", 1, inputLabels, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Verify the created bead has the labels
	if len(created.Labels) != len(inputLabels) {
		t.Errorf("Created bead labels mismatch: got %v, want %v", created.Labels, inputLabels)
	}
	for i, label := range inputLabels {
		if i >= len(created.Labels) || created.Labels[i] != label {
			t.Errorf("Label %d mismatch: got %q, want %q", i, created.Labels[i], label)
		}
	}
}


func TestTrackerClientAdapter_CreateExpectedOutputsMetadata(t *testing.T) {
	t.Parallel()

	mockClient := &mockTrackerClient{}
	adapter := &trackerClientAdapter{Client: mockClient}

	ctx := context.Background()
	outputs := []string{"output-one", "output-two"}
	if _, err := adapter.Create(ctx, "title", 1, nil, outputs); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if len(mockClient.createdItems) == 0 {
		t.Fatal("no created items captured")
	}
	raw := mockClient.createdItems[0].Metadata["expected_outputs"]
	if raw == "" {
		t.Fatal("expected_outputs metadata is empty")
	}
	var decoded []string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if len(decoded) != len(outputs) {
		t.Fatalf("decoded outputs length = %d, want %d", len(decoded), len(outputs))
	}
	for i, v := range outputs {
		if decoded[i] != v {
			t.Fatalf("output[%d] = %q, want %q", i, decoded[i], v)
		}
	}
}
