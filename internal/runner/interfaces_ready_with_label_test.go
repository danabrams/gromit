package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestBeadClient_HasReadyWithLabel verifies that BeadClient interface includes ReadyWithLabel method
func TestBeadClient_HasReadyWithLabel(t *testing.T) {
	// This test will fail to compile if ReadyWithLabel is not in the BeadClient interface
	var _ BeadClient = (*bead.Client)(nil)

	// Create a concrete client
	client, err := bead.NewClient()
	if err != nil {
		t.Fatalf("Failed to create bead client: %v", err)
	}

	// Verify ReadyWithLabel method exists and has correct signature
	// This will fail to compile if the method doesn't exist on bead.Client
	_, err = client.ReadyWithLabel("spec:test")
	// We expect an error since bd isn't running, but the method should exist
	if err == nil {
		t.Log("ReadyWithLabel method exists (no error because bd might be running)")
	} else {
		t.Logf("ReadyWithLabel method exists (expected error: %v)", err)
	}
}

// TestBeadClient_InterfaceSatisfaction verifies that bead.Client satisfies BeadClient interface
// with the new ReadyWithLabel method
func TestBeadClient_InterfaceSatisfaction(t *testing.T) {
	// This test ensures that the compile-time interface check in interfaces.go
	// will catch if ReadyWithLabel is missing from bead.Client
	//
	// The actual check is:
	//   var _ BeadClient = (*bead.Client)(nil)
	//
	// Which is declared at the package level in interfaces.go
	t.Log("✓ bead.Client satisfies BeadClient interface (verified at compile time)")
}

// MockBeadClientWithLabel is a mock that implements BeadClient including ReadyWithLabel
type MockBeadClientWithLabel struct {
	ReadyFunc                          func() (*bead.Bead, error)
	ReadyWithLabelFunc                 func(label string) (*bead.Bead, error)
	ListWithLabelFunc                  func(label string) ([]*bead.Bead, error)
	ShowFunc                           func(id string) (*bead.Bead, error)
	CloseFunc                          func(id string) error
	SyncFunc                           func() error
	AddCommentFunc                     func(id, comment string) error
	GetParentFunc                      func(b *bead.Bead) (*bead.Bead, error)
	CreateWithParentFunc               func(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error)
	CreateWithParentAndDescriptionFunc func(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error)
	HasOpenChildrenFunc                func(parentID string) (bool, error)
}

func (m *MockBeadClientWithLabel) Ready() (*bead.Bead, error) {
	if m.ReadyFunc != nil {
		return m.ReadyFunc()
	}
	return nil, nil
}

func (m *MockBeadClientWithLabel) ReadyWithLabel(label string) (*bead.Bead, error) {
	if m.ReadyWithLabelFunc != nil {
		return m.ReadyWithLabelFunc(label)
	}
	return nil, nil
}

func (m *MockBeadClientWithLabel) ListWithLabel(label string) ([]*bead.Bead, error) {
	if m.ListWithLabelFunc != nil {
		return m.ListWithLabelFunc(label)
	}
	return []*bead.Bead{}, nil
}

func (m *MockBeadClientWithLabel) Show(id string) (*bead.Bead, error) {
	if m.ShowFunc != nil {
		return m.ShowFunc(id)
	}
	return nil, nil
}

func (m *MockBeadClientWithLabel) Close(id string) error {
	if m.CloseFunc != nil {
		return m.CloseFunc(id)
	}
	return nil
}

func (m *MockBeadClientWithLabel) Sync() error {
	if m.SyncFunc != nil {
		return m.SyncFunc()
	}
	return nil
}

func (m *MockBeadClientWithLabel) AddComment(id, comment string) error {
	if m.AddCommentFunc != nil {
		return m.AddCommentFunc(id, comment)
	}
	return nil
}

func (m *MockBeadClientWithLabel) GetParent(b *bead.Bead) (*bead.Bead, error) {
	if m.GetParentFunc != nil {
		return m.GetParentFunc(b)
	}
	return nil, nil
}

func (m *MockBeadClientWithLabel) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	if m.CreateWithParentFunc != nil {
		return m.CreateWithParentFunc(title, priority, labels, expectedOutputs, parentID)
	}
	return nil, nil
}

func (m *MockBeadClientWithLabel) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	if m.CreateWithParentAndDescriptionFunc != nil {
		return m.CreateWithParentAndDescriptionFunc(title, priority, labels, expectedOutputs, parentID, description)
	}
	return nil, nil
}

func (m *MockBeadClientWithLabel) HasOpenChildren(parentID string) (bool, error) {
	if m.HasOpenChildrenFunc != nil {
		return m.HasOpenChildrenFunc(parentID)
	}
	return false, nil
}

// TestMockBeadClientWithLabel_ImplementsInterface verifies mock satisfies interface
func TestMockBeadClientWithLabel_ImplementsInterface(t *testing.T) {
	var _ BeadClient = (*MockBeadClientWithLabel)(nil)
	t.Log("✓ MockBeadClientWithLabel satisfies BeadClient interface")
}

// TestMockBeadClientWithLabel_ReadyWithLabelFunctionality tests the mock's ReadyWithLabel
func TestMockBeadClientWithLabel_ReadyWithLabelFunctionality(t *testing.T) {
	expectedBead := &bead.Bead{
		ID:       "test-001",
		Title:    "Test task",
		Priority: 1,
		Labels:   []string{"spec:auth"},
		Type:     "task",
		Status:   "open",
	}

	mock := &MockBeadClientWithLabel{
		ReadyWithLabelFunc: func(label string) (*bead.Bead, error) {
			if label == "spec:auth" {
				return expectedBead, nil
			}
			return nil, nil
		},
	}

	// Test with matching label
	result, err := mock.ReadyWithLabel("spec:auth")
	if err != nil {
		t.Errorf("ReadyWithLabel() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("ReadyWithLabel() returned nil bead")
	}
	if result.ID != expectedBead.ID {
		t.Errorf("ReadyWithLabel() ID = %v, want %v", result.ID, expectedBead.ID)
	}

	// Test with non-matching label
	result, err = mock.ReadyWithLabel("spec:payments")
	if err != nil {
		t.Errorf("ReadyWithLabel() unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("ReadyWithLabel() expected nil for non-matching label, got %+v", result)
	}
}
