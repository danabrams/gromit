package tasktracker

import "context"

// Bead represents a task tracked by bd
type Bead struct {
	ID          string
	Title       string
	Description string
	Priority    int
	Labels      []string
	Status      string
	// Dependencies info
	DependsOn  []string // IDs of beads this depends on
	BlockedBy  []string // IDs of beads blocking this
	Dependents []string // IDs of beads depending on this
}

// TaskTrackerNextBeadRequest defines filters used to select the next bead.
type TaskTrackerNextBeadRequest struct {
	Labels []string
}

// TaskTrackerNextBeadResponse wraps the bead that should be worked on next.
type TaskTrackerNextBeadResponse struct {
	Bead *Bead
}

// NextBeadRequest is retained for backwards compatibility with existing callers.
type NextBeadRequest = TaskTrackerNextBeadRequest

// NextBeadResponse is retained for backwards compatibility with existing callers.
type NextBeadResponse = TaskTrackerNextBeadResponse

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

// CreateBeadRequest is retained for backwards compatibility with existing callers.
type CreateBeadRequest = TaskTrackerCreateBeadRequest

// CreateBeadResponse is retained for backwards compatibility with existing callers.
type CreateBeadResponse = TaskTrackerCreateBeadResponse

// TaskTrackerCloseBeadRequest identifies which bead should be closed.
type TaskTrackerCloseBeadRequest struct {
	BeadID string
}

// TaskTrackerCloseBeadResponse reports whether the bead was closed.
type TaskTrackerCloseBeadResponse struct {
	Closed bool
}

// CloseBeadRequest is retained for backwards compatibility with existing callers.
type CloseBeadRequest = TaskTrackerCloseBeadRequest

// CloseBeadResponse is retained for backwards compatibility with existing callers.
type CloseBeadResponse = TaskTrackerCloseBeadResponse

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

// QueryBeadsRequest is retained for backwards compatibility with existing callers.
type QueryBeadsRequest = TaskTrackerQueryBeadsRequest

// QueryBeadsResponse is retained for backwards compatibility with existing callers.
type QueryBeadsResponse = TaskTrackerQueryBeadsResponse

// TaskTracker provides operations for querying and managing tasks via bd
type TaskTracker interface {
	// NextBead returns the next open bead with dependency information
	NextBead(ctx context.Context, req TaskTrackerNextBeadRequest) (*TaskTrackerNextBeadResponse, error)

	// ShowBead returns metadata for the specified bead.
	ShowBead(ctx context.Context, beadID string) (*Bead, error)

	// CreateBead creates a new bead with the given properties
	// It automatically adds a gen:N label and declares dependencies
	CreateBead(ctx context.Context, req TaskTrackerCreateBeadRequest) (*TaskTrackerCreateBeadResponse, error)

	// CloseBead marks a bead as closed
	CloseBead(ctx context.Context, req TaskTrackerCloseBeadRequest) (*TaskTrackerCloseBeadResponse, error)

	// QueryBeads filters beads by labels, status, and parent
	QueryBeads(ctx context.Context, req TaskTrackerQueryBeadsRequest) (*TaskTrackerQueryBeadsResponse, error)
}
