package tasktracker

import (
	"context"
	"testing"
)

type taskTrackerTestStub struct{}

func (taskTrackerTestStub) NextBead(ctx context.Context, req NextBeadRequest) (*NextBeadResponse, error) {
	return &NextBeadResponse{Bead: &Bead{ID: "test"}}, nil
}

func (taskTrackerTestStub) CreateBead(ctx context.Context, req CreateBeadRequest) (*CreateBeadResponse, error) {
	return &CreateBeadResponse{Bead: &Bead{ID: "created"}}, nil
}

func (taskTrackerTestStub) CloseBead(ctx context.Context, req CloseBeadRequest) (*CloseBeadResponse, error) {
	return &CloseBeadResponse{Closed: true}, nil
}

func (taskTrackerTestStub) QueryBeads(ctx context.Context, req QueryBeadsRequest) (*QueryBeadsResponse, error) {
	return &QueryBeadsResponse{}, nil
}

func (taskTrackerTestStub) ShowBead(context.Context, string) (*Bead, error) {
	return &Bead{ID: "stub"}, nil
}

func TestTaskTrackerInterface(t *testing.T) {
	var _ TaskTracker = taskTrackerTestStub{}
}
