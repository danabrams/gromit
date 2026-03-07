package tasktracker

import (
	"context"
	"testing"
)

func TestTaskTrackerContract(t *testing.T) {
	var _ interface {
		NextBead(context.Context, TaskTrackerNextBeadRequest) (*TaskTrackerNextBeadResponse, error)
		CreateBead(context.Context, TaskTrackerCreateBeadRequest) (*TaskTrackerCreateBeadResponse, error)
		CloseBead(context.Context, TaskTrackerCloseBeadRequest) (*TaskTrackerCloseBeadResponse, error)
		QueryBeads(context.Context, TaskTrackerQueryBeadsRequest) (*TaskTrackerQueryBeadsResponse, error)
	} = (TaskTracker)(nil)
}
