package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestAcceptance_AllMockBeadClientsImplementInterface verifies that all mock
// BeadClient implementations in the internal/runner package satisfy the
// BeadClient interface, including ReadyWithLabel and ListWithLabel methods.
func TestAcceptance_AllMockBeadClientsImplementInterface(t *testing.T) {
	tests := []struct {
		name string
		mock BeadClient
	}{
		{
			name: "mockBeadClient satisfies BeadClient interface",
			mock: &mockBeadClient{},
		},
		{
			name: "mockBeadClientForStatus satisfies BeadClient interface",
			mock: &mockBeadClientForStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail to compile if the mock doesn't satisfy the interface
			var _ BeadClient = tt.mock

			// Verify ReadyWithLabel can be called
			_, err := tt.mock.ReadyWithLabel("test-label")
			if err != nil {
				// Error is acceptable - we're just verifying the method exists
			}

			// Verify ListWithLabel can be called
			_, err = tt.mock.ListWithLabel("test-label")
			if err != nil {
				// Error is acceptable - we're just verifying the method exists
			}
		})
	}
}

// TestAcceptance_MockBeadClientReadyWithLabelIsCallable verifies that
// mockBeadClient's ReadyWithLabel method is callable and returns expected values.
func TestAcceptance_MockBeadClientReadyWithLabelIsCallable(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func() *mockBeadClient
		label      string
		wantBeadID string
		wantErr    bool
	}{
		{
			name: "nil callback returns nil bead and no error",
			setupMock: func() *mockBeadClient {
				return &mockBeadClient{}
			},
			label:      "spec:test",
			wantBeadID: "",
			wantErr:    false,
		},
		{
			name: "custom callback returns configured bead",
			setupMock: func() *mockBeadClient {
				return &mockBeadClient{
					ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
						return &bead.Bead{ID: "test-123", Title: "Test Bead"}, nil
					},
				}
			},
			label:      "spec:auth",
			wantBeadID: "test-123",
			wantErr:    false,
		},
		{
			name: "custom callback can filter by label",
			setupMock: func() *mockBeadClient {
				return &mockBeadClient{
					ReadyWithLabelFn: func(label string) (*bead.Bead, error) {
						if label == "spec:payments" {
							return &bead.Bead{ID: "pay-001", Title: "Payment Task"}, nil
						}
						return nil, nil
					},
				}
			},
			label:      "spec:payments",
			wantBeadID: "pay-001",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.setupMock()
			result, err := mock.ReadyWithLabel(tt.label)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadyWithLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantBeadID == "" {
				if result != nil {
					t.Errorf("ReadyWithLabel() returned bead %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("ReadyWithLabel() returned nil, want bead with ID %s", tt.wantBeadID)
					return
				}
				if result.ID != tt.wantBeadID {
					t.Errorf("ReadyWithLabel() bead ID = %s, want %s", result.ID, tt.wantBeadID)
				}
			}
		})
	}
}

// TestAcceptance_MockBeadClientListWithLabelIsCallable verifies that
// mockBeadClient's ListWithLabel method is callable and returns expected values.
func TestAcceptance_MockBeadClientListWithLabelIsCallable(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func() *mockBeadClient
		label     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "nil callback returns empty slice and no error",
			setupMock: func() *mockBeadClient {
				return &mockBeadClient{}
			},
			label:     "spec:test",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "custom callback returns configured beads",
			setupMock: func() *mockBeadClient {
				return &mockBeadClient{
					ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
						return []*bead.Bead{
							{ID: "task-1", Title: "First Task"},
							{ID: "task-2", Title: "Second Task"},
						}, nil
					},
				}
			},
			label:     "spec:auth",
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "custom callback can filter by label",
			setupMock: func() *mockBeadClient {
				return &mockBeadClient{
					ListWithLabelFn: func(label string) ([]*bead.Bead, error) {
						if label == "spec:payments" {
							return []*bead.Bead{
								{ID: "pay-001", Title: "Payment Task 1"},
								{ID: "pay-002", Title: "Payment Task 2"},
								{ID: "pay-003", Title: "Payment Task 3"},
							}, nil
						}
						return []*bead.Bead{}, nil
					},
				}
			},
			label:     "spec:payments",
			wantCount: 3,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := tt.setupMock()
			results, err := mock.ListWithLabel(tt.label)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListWithLabel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if results == nil {
				t.Error("ListWithLabel() returned nil slice, want empty slice")
				return
			}

			if len(results) != tt.wantCount {
				t.Errorf("ListWithLabel() returned %d beads, want %d", len(results), tt.wantCount)
			}
		})
	}
}

// TestAcceptance_MockBeadClientForStatusImplementsMethods verifies that
// mockBeadClientForStatus has stub implementations of ReadyWithLabel and
// ListWithLabel that don't break existing code.
func TestAcceptance_MockBeadClientForStatusImplementsMethods(t *testing.T) {
	mock := &mockBeadClientForStatus{
		ready: &bead.Bead{ID: "test-bead"},
	}

	// Verify ReadyWithLabel is callable and returns safe defaults
	bead, err := mock.ReadyWithLabel("any-label")
	if err != nil {
		t.Errorf("ReadyWithLabel() returned unexpected error: %v", err)
	}
	if bead != nil {
		t.Errorf("ReadyWithLabel() returned %v, want nil (stub implementation)", bead)
	}

	// Verify ListWithLabel is callable and returns safe defaults
	beads, err := mock.ListWithLabel("any-label")
	if err != nil {
		t.Errorf("ListWithLabel() returned unexpected error: %v", err)
	}
	if beads == nil {
		t.Error("ListWithLabel() returned nil, want empty slice")
	}
	if len(beads) != 0 {
		t.Errorf("ListWithLabel() returned %d beads, want 0 (stub implementation)", len(beads))
	}
}

// TestAcceptance_MocksCanBeUsedThroughInterface verifies that all mocks
// can be used through the BeadClient interface without compilation errors
// and that ReadyWithLabel/ListWithLabel are accessible.
func TestAcceptance_MocksCanBeUsedThroughInterface(t *testing.T) {
	mocks := []struct {
		name   string
		client BeadClient
	}{
		{
			name:   "mockBeadClient",
			client: &mockBeadClient{},
		},
		{
			name:   "mockBeadClientForStatus",
			client: &mockBeadClientForStatus{},
		},
	}

	for _, m := range mocks {
		t.Run(m.name, func(t *testing.T) {
			// These calls verify the methods exist on the interface
			// and are accessible through the BeadClient interface type
			_, _ = m.client.ReadyWithLabel("test")
			_, _ = m.client.ListWithLabel("test")

			// Also verify the traditional methods still work
			_, _ = m.client.Ready()
			_, _ = m.client.Show("test-id")
		})
	}
}
