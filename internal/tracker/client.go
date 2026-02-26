package tracker

import "context"

// Client defines the operations that a tracker backend must provide.
type Client interface {
	Ready(ctx context.Context) (*Item, error)
	List(ctx context.Context, query Query) ([]Item, error)
	Show(ctx context.Context, id string) (*Item, error)
	Create(ctx context.Context, req CreateRequest) (*Item, error)
	CreateWithParent(ctx context.Context, req CreateRequest, parentID string) (*Item, error)
	ListWithLabel(ctx context.Context, label string) ([]Item, error)
	Close(ctx context.Context, id string) error
	Sync(ctx context.Context) error
	AddComment(ctx context.Context, id, comment string) error
	HasOpenChildren(ctx context.Context, parentID string) (bool, error)
}

// ItemReader defines item-fetching operations for tracker backends.
type ItemReader interface {
	// Ready returns the next available item for work, or nil when none exist.
	Ready(ctx context.Context) (*Item, error)
	// List returns tracker items matching the provided query.
	List(ctx context.Context, query Query) ([]Item, error)
	// Show returns full details for the requested item.
	Show(ctx context.Context, id string) (*Item, error)
	// ListWithLabel returns tracker items that have the provided label.
	ListWithLabel(ctx context.Context, label string) ([]Item, error)
}

// ItemWriter defines item-creation and mutation operations for tracker backends.
type ItemWriter interface {
	// Create adds a new tracker item from the supplied request.
	Create(ctx context.Context, req CreateRequest) (*Item, error)
	// CreateWithParent adds a new tracker item with a parent reference.
	CreateWithParent(ctx context.Context, req CreateRequest, parentID string) (*Item, error)
	// Update modifies an existing tracker item.
	Update(ctx context.Context, req UpdateRequest) (*Item, error)
	// Close marks an item as completed.
	Close(ctx context.Context, id string) error
	// Sync ensures the tracker backend is in sync with its source of truth.
	Sync(ctx context.Context) error
}
