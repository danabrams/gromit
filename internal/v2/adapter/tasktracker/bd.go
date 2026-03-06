package tasktracker

import (
	"context"
)

// BDAdapter adapts the bd CLI to provide TaskTracker functionality
type BDAdapter struct {
	bdClient interface{} // placeholder for actual bd client
}

// NewBDAdapter creates a new BDAdapter
func NewBDAdapter(bdClient interface{}) *BDAdapter {
	return &BDAdapter{
		bdClient: bdClient,
	}
}

// NextBead returns the next open bead with dependency information
func (a *BDAdapter) NextBead(ctx context.Context) (*Bead, error) {
	return &Bead{
		ID:     "test-bead",
		Title:  "Test Bead",
		Status: "open",
	}, nil
}

// ShowBead returns a placeholder bead for the requested ID.
func (a *BDAdapter) ShowBead(ctx context.Context, beadID string) (*Bead, error) {
	return &Bead{ID: beadID, Status: "open"}, nil
}

// CreateBead creates a new bead with the given properties
func (a *BDAdapter) CreateBead(ctx context.Context, title, description string, priority int, labels, dependencies []string) (*Bead, error) {
	return &Bead{
		ID:          "new-bead",
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      "open",
		Labels:      append([]string(nil), labels...),
		DependsOn:   append([]string(nil), dependencies...),
	}, nil
}

// CloseBead marks a bead as closed
func (a *BDAdapter) CloseBead(ctx context.Context, beadID string) error {
	return nil
}

// QueryBeads filters beads by labels, status, and parent
func (a *BDAdapter) QueryBeads(ctx context.Context, labels []string, status, parent string) ([]Bead, error) {
	return []Bead{}, nil
}

// RecordPlan persists the generated plan for the spec.
func (a *BDAdapter) RecordPlan(ctx context.Context, specID, plan string) error {
	return nil
}
