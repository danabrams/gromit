package bead

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/tracker"
)

// BDAdapter exposes bead.Client through the tracker.Client contract.
type BDAdapter struct {
	client *Client
}

var _ tracker.Client = (*BDAdapter)(nil)

// NewBDAdapter creates a new BDAdapter wrapping the given bead.Client.
func NewBDAdapter(c *Client) *BDAdapter {
	return &BDAdapter{client: c}
}

// UnwrapBDAdapter extracts the underlying *bead.Client from a tracker.Client if it's a BDAdapter.
// Returns nil if the client is not a BDAdapter.
func UnwrapBDAdapter(tc tracker.Client) *Client {
	if adapter, ok := tc.(*BDAdapter); ok && adapter != nil {
		return adapter.client
	}
	return nil
}

func (a *BDAdapter) clientOrErr() (*Client, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("bd adapter client is nil")
	}
	return a.client, nil
}

func (a *BDAdapter) Ready(ctx context.Context) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	bead, err := client.Ready()
	if err != nil {
		return nil, err
	}
	return beadToItem(bead), nil
}

func (a *BDAdapter) List(ctx context.Context, query tracker.Query) ([]tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	statuses := query.Filter.Statuses
	if len(statuses) == 0 {
		statuses = []string{"open"}
	}

	seen := make(map[string]struct{})
	var items []tracker.Item

	for _, status := range statuses {
		status = strings.TrimSpace(status)
		if status == "" {
			continue
		}
		beads, err := client.ListByStatus(strings.ToLower(status))
		if err != nil {
			return nil, err
		}
		for _, b := range beads {
			if b == nil {
				continue
			}
			if _, ok := seen[b.ID]; ok {
				continue
			}
			seen[b.ID] = struct{}{}
			if item := beadToItem(b); item != nil {
				items = append(items, *item)
			}
		}
	}

	start := query.Offset
	if start < 0 {
		start = 0
	}
	if start >= len(items) {
		return []tracker.Item{}, nil
	}

	end := len(items)
	if query.Limit > 0 && start+query.Limit < end {
		end = start + query.Limit
	}

	return items[start:end], nil
}

func (a *BDAdapter) Show(ctx context.Context, id string) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	bead, err := client.Show(id)
	if err != nil {
		return nil, err
	}
	return beadToItem(bead), nil
}

func (a *BDAdapter) Create(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	priority, labels, expectedOutputs, parent, paramsErr := createParamsFromRequest(req)
	if paramsErr != nil {
		return nil, paramsErr
	}

	bead, err := client.CreateWithParentAndDescription(req.Title, priority, labels, expectedOutputs, parent, req.Description)
	if err != nil {
		return nil, err
	}
	return beadToItem(bead), nil
}

func (a *BDAdapter) Close(ctx context.Context, id string) error {
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	return client.Close(id)
}

func (a *BDAdapter) Sync(ctx context.Context) error {
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	return client.Sync()
}

func (a *BDAdapter) AddComment(ctx context.Context, id, comment string) error {
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	return client.AddComment(id, comment)
}

func (a *BDAdapter) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return false, err
	}
	return client.HasOpenChildren(parentID)
}
