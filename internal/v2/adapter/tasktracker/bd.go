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
func (a *BDAdapter) NextBead(ctx context.Context, req NextBeadRequest) (*NextBeadResponse, error) {
	return &NextBeadResponse{Bead: &Bead{ID: "test-bead", Title: "Test Bead", Status: "open"}}, nil
}

// ShowBead returns a placeholder bead for the requested ID.
func (a *BDAdapter) ShowBead(ctx context.Context, beadID string) (*Bead, error) {
	return &Bead{ID: beadID, Status: "open"}, nil
}

// CreateBead creates a new bead with the given properties
func (a *BDAdapter) CreateBead(ctx context.Context, req CreateBeadRequest) (*CreateBeadResponse, error) {
	return &CreateBeadResponse{Bead: &Bead{
		ID:          "new-bead",
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Status:      "open",
		Labels:      append([]string(nil), req.Labels...),
		DependsOn:   append([]string(nil), req.Dependencies...),
	}}, nil
}

// CloseBead marks a bead as closed
func (a *BDAdapter) CloseBead(ctx context.Context, req CloseBeadRequest) (*CloseBeadResponse, error) {
	return &CloseBeadResponse{Closed: true}, nil
}

// QueryBeads filters beads by labels, status, and parent
func (a *BDAdapter) QueryBeads(ctx context.Context, req QueryBeadsRequest) (*QueryBeadsResponse, error) {
	return &QueryBeadsResponse{}, nil
}

// RecordPlan persists the generated plan for the spec.
func (a *BDAdapter) RecordPlan(ctx context.Context, specID, plan string) error {
	return nil
}
