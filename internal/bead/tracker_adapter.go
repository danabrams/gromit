package bead

import (
	"context"
	"fmt"
	"strconv"
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

	bead, err := client.Ready(ctx)
	if err != nil {
		return nil, err
	}
	return beadToItem(bead), nil
}

func (a *BDAdapter) ReadyWithLabel(ctx context.Context, label string) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	beads, err := client.ListWithLabel(ctx, label)
	if err != nil {
		return nil, err
	}

	for _, b := range beads {
		if b == nil {
			continue
		}
		if strings.EqualFold(b.Type, "epic") {
			continue
		}
		if strings.EqualFold(b.Status, tracker.StatusClosed) {
			continue
		}
		if item := beadToItem(b); item != nil {
			return item, nil
		}
	}

	return nil, nil
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
		beads, err := client.ListByStatus(ctx, strings.ToLower(status))
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
			// Filter by labels if specified
			if len(query.Filter.Labels) > 0 && !beadHasAnyLabel(b, query.Filter.Labels) {
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

func (a *BDAdapter) Search(ctx context.Context, query tracker.Query) ([]tracker.Item, error) {
	return a.List(ctx, query)
}

func (a *BDAdapter) ListWithLabel(ctx context.Context, label string) ([]tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	beads, err := client.ListWithLabel(ctx, label)
	if err != nil {
		return nil, err
	}

	items := make([]tracker.Item, 0, len(beads))
	for _, b := range beads {
		if item := beadToItem(b); item != nil {
			items = append(items, *item)
		}
	}

	return items, nil
}

// beadHasAnyLabel checks if a bead has at least one of the specified labels
func beadHasAnyLabel(b *Bead, requiredLabels []string) bool {
	if b == nil || len(requiredLabels) == 0 {
		return true
	}
	for _, required := range requiredLabels {
		for _, beadLabel := range b.Labels {
			if beadLabel == required {
				return true
			}
		}
	}
	return false
}

func parsePriority(metadata map[string]string) (int, bool, error) {
	if metadata == nil {
		return 0, false, nil
	}
	raw, ok := metadata["priority"]
	if !ok {
		return 0, false, nil
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, nil
	}

	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false, fmt.Errorf("invalid priority %q: %w", raw, err)
	}

	return value, true, nil
}

func (a *BDAdapter) Show(ctx context.Context, id string) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	bead, err := client.Show(ctx, id)
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

	bead, err := client.CreateWithParentAndDescription(ctx, req.Title, priority, labels, expectedOutputs, parent, req.Description)
	if err != nil {
		return nil, err
	}
	return beadToItem(bead), nil
}

func (a *BDAdapter) CreateWithParent(ctx context.Context, req tracker.CreateRequest, parentID string) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	priority, labels, expectedOutputs, parent, paramsErr := createParamsFromRequest(req)
	if paramsErr != nil {
		return nil, paramsErr
	}

	if trimmed := strings.TrimSpace(parentID); trimmed != "" {
		parent = trimmed
	}

	bead, err := client.CreateWithParentAndDescription(ctx, req.Title, priority, labels, expectedOutputs, parent, req.Description)
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
	return client.Close(ctx, id)
}

func (a *BDAdapter) Sync(ctx context.Context) error {
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	return client.Sync(ctx)
}

func (a *BDAdapter) Update(ctx context.Context, req tracker.UpdateRequest) (*tracker.Item, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ID) == "" {
		return nil, fmt.Errorf("update request must include an ID")
	}

	if priority, ok, parseErr := parsePriority(req.Metadata); parseErr != nil {
		return nil, parseErr
	} else if ok {
		if err := client.UpdatePriority(ctx, req.ID, priority); err != nil {
			return nil, err
		}
	}

	return a.Show(ctx, req.ID)
}

func (a *BDAdapter) AddComment(ctx context.Context, id, comment string) error {
	client, err := a.clientOrErr()
	if err != nil {
		return err
	}
	return client.AddComment(ctx, id, comment)
}

func (a *BDAdapter) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return false, err
	}
	return client.HasOpenChildren(ctx, parentID)
}
