package tracker

import "context"

// Client defines the operations that a tracker backend must provide.
type Client interface {
	// List returns tracker items matching the provided query.
	List(ctx context.Context, query Query) ([]Item, error)
	// Get fetches a single tracker item by its identifier.
	Get(ctx context.Context, id string) (*Item, error)
	// Create adds a new tracker item from the supplied request.
	Create(ctx context.Context, req CreateRequest) (*Item, error)
	// UpdateStatus adjusts the status of an existing tracker item.
	UpdateStatus(ctx context.Context, id, status string) error
}
