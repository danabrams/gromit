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

// TaskTracker provides operations for querying and managing tasks via bd
type TaskTracker interface {
	// NextBead returns the next open bead with dependency information
	NextBead(ctx context.Context) (*Bead, error)

	// CreateBead creates a new bead with the given properties
	// It automatically adds a gen:N label and declares dependencies
	CreateBead(ctx context.Context, title, description string, priority int, dependencies []string) (*Bead, error)

	// CloseBead marks a bead as closed
	CloseBead(ctx context.Context, beadID string) error

	// QueryBeads filters beads by labels, status, and parent
	QueryBeads(ctx context.Context, labels []string, status, parent string) ([]Bead, error)
}
