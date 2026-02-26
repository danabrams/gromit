package tracker

import "context"

// Client defines the operations that a tracker backend must provide.
type Client interface {
	// Ready returns the next available item for work, or nil when none exist.
	Ready(ctx context.Context) (*Item, error)
	// List returns tracker items matching the provided query.
	List(ctx context.Context, query Query) ([]Item, error)
	// Show returns full details for the requested item.
	Show(ctx context.Context, id string) (*Item, error)
	// Create adds a new tracker item from the supplied request.
	Create(ctx context.Context, req CreateRequest) (*Item, error)
	// CreateWithParent adds a new tracker item with a parent reference.
	CreateWithParent(ctx context.Context, req CreateRequest, parentID string) (*Item, error)
	// ListWithLabel returns tracker items that have the provided label.
	ListWithLabel(ctx context.Context, label string) ([]Item, error)
	// Close marks an item as completed.
	Close(ctx context.Context, id string) error
	// Sync ensures the tracker backend is in sync with its source of truth.
	Sync(ctx context.Context) error
	// AddComment attaches a note to an item.
	AddComment(ctx context.Context, id, comment string) error
	// HasOpenChildren reports whether the provided parent still has outstanding children.
	HasOpenChildren(ctx context.Context, parentID string) (bool, error)
}
