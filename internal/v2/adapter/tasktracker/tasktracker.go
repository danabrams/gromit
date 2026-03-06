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

// NextBeadRequest defines filters used to select the next bead.
type NextBeadRequest struct {
	Labels []string
}

// NextBeadResponse wraps the bead that should be worked on next.
type NextBeadResponse struct {
	Bead *Bead
}

// CreateBeadRequest carries the properties for a new bead.
type CreateBeadRequest struct {
	Title        string
	Description  string
	Priority     int
	Labels       []string
	Dependencies []string
}

// CreateBeadResponse returns the bead that was created.
type CreateBeadResponse struct {
	Bead *Bead
}

// CloseBeadRequest identifies which bead should be closed.
type CloseBeadRequest struct {
	BeadID string
}

// CloseBeadResponse reports whether the bead was closed.
type CloseBeadResponse struct {
	Closed bool
}

// QueryBeadsRequest filters the bead set.
type QueryBeadsRequest struct {
	Labels []string
	Status string
	Parent string
}

// QueryBeadsResponse returns beads that match the filters.
type QueryBeadsResponse struct {
	Beads []Bead
}

// TaskTracker provides operations for querying and managing tasks via bd
type TaskTracker interface {
	// NextBead returns the next open bead with dependency information
	NextBead(ctx context.Context, req NextBeadRequest) (*NextBeadResponse, error)

	// ShowBead returns metadata for the specified bead.
	ShowBead(ctx context.Context, beadID string) (*Bead, error)

	// CreateBead creates a new bead with the given properties
	// It automatically adds a gen:N label and declares dependencies
	CreateBead(ctx context.Context, req CreateBeadRequest) (*CreateBeadResponse, error)

	// CloseBead marks a bead as closed
	CloseBead(ctx context.Context, req CloseBeadRequest) (*CloseBeadResponse, error)

	// QueryBeads filters beads by labels, status, and parent
	QueryBeads(ctx context.Context, req QueryBeadsRequest) (*QueryBeadsResponse, error)
}
