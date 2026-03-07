package trackertypes

import "context"

// Bead represents a task tracked by bd.
type Bead struct {
	ID          string
	Title       string
	Description string
	Priority    int
	Labels      []string
	Status      string
	DependsOn   []string
	BlockedBy   []string
	Dependents  []string
}

// TaskTrackerNextBeadRequest defines filters used to select the next bead.
type TaskTrackerNextBeadRequest struct {
	Labels []string
}

// TaskTrackerNextBeadResponse wraps the bead that should be worked on next.
type TaskTrackerNextBeadResponse struct {
	Bead *Bead
}

// TaskTrackerCreateBeadRequest carries the properties for a new bead.
type TaskTrackerCreateBeadRequest struct {
	Title        string
	Description  string
	Priority     int
	Labels       []string
	Dependencies []string
}

// TaskTrackerCreateBeadResponse returns the bead that was created.
type TaskTrackerCreateBeadResponse struct {
	Bead *Bead
}

// TaskTrackerCloseBeadRequest identifies which bead should be closed.
type TaskTrackerCloseBeadRequest struct {
	BeadID string
}

// TaskTrackerCloseBeadResponse reports whether the bead was closed.
type TaskTrackerCloseBeadResponse struct {
	Closed bool
}

// TaskTrackerQueryBeadsRequest filters the bead set.
type TaskTrackerQueryBeadsRequest struct {
	Labels []string
	Status string
	Parent string
}

// TaskTrackerQueryBeadsResponse returns beads that match the filters.
type TaskTrackerQueryBeadsResponse struct {
	Beads []Bead
}

// TaskTracker provides operations for querying and managing tasks via bd.
type TaskTracker interface {
	NextBead(ctx context.Context, req TaskTrackerNextBeadRequest) (*TaskTrackerNextBeadResponse, error)
	ShowBead(ctx context.Context, beadID string) (*Bead, error)
	CreateBead(ctx context.Context, req TaskTrackerCreateBeadRequest) (*TaskTrackerCreateBeadResponse, error)
	CloseBead(ctx context.Context, req TaskTrackerCloseBeadRequest) (*TaskTrackerCloseBeadResponse, error)
	QueryBeads(ctx context.Context, req TaskTrackerQueryBeadsRequest) (*TaskTrackerQueryBeadsResponse, error)
}
