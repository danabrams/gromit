package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/tracker"
)

type trackerBeadClient struct {
	client tracker.Client
}

func newTrackerBeadClient(client tracker.Client) BeadClient {
	if client == nil {
		return nil
	}
	return &trackerBeadClient{client: client}
}

func (c *trackerBeadClient) Ready(ctx context.Context) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	item, err := c.client.Ready(ctx)
	if err != nil {
		return nil, err
	}
	return bead.TrackerItemToBead(item)
}

func (c *trackerBeadClient) ReadyExcluding(ctx context.Context, excludeIDs map[string]bool) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if len(excludeIDs) == 0 {
		return c.Ready(ctx)
	}

	items, err := c.client.List(ctx, tracker.Query{
		Filter: tracker.Filter{Statuses: []string{tracker.StatusOpen, tracker.StatusInProgress}},
		Limit:  0,
	})
	if err != nil {
		return nil, err
	}
	for i := range items {
		if excludeIDs[items[i].ID] {
			continue
		}
		beadObj, err := bead.TrackerItemToBead(&items[i])
		if err != nil {
			return nil, err
		}
		if beadObj == nil {
			continue
		}
		if strings.EqualFold(beadObj.Type, "epic") {
			continue
		}
		if strings.EqualFold(beadObj.Status, tracker.StatusClosed) {
			continue
		}
		return beadObj, nil
	}
	return nil, nil
}

func (c *trackerBeadClient) ReadyWithLabel(ctx context.Context, label string) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	items, err := c.client.ListWithLabel(ctx, label)
	if err != nil {
		return nil, err
	}
	for i := range items {
		beadObj, err := bead.TrackerItemToBead(&items[i])
		if err != nil {
			return nil, err
		}
		if beadObj == nil {
			continue
		}
		if strings.EqualFold(beadObj.Type, "epic") {
			continue
		}
		if strings.EqualFold(beadObj.Status, tracker.StatusClosed) {
			continue
		}
		return beadObj, nil
	}
	return nil, nil
}

func (c *trackerBeadClient) ListWithLabel(ctx context.Context, label string) ([]*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	items, err := c.client.ListWithLabel(ctx, label)
	if err != nil {
		return nil, err
	}
	return toBeads(items)
}

func (c *trackerBeadClient) Show(ctx context.Context, id string) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	item, err := c.client.Show(ctx, id)
	if err != nil {
		return nil, err
	}
	return bead.TrackerItemToBead(item)
}

func (c *trackerBeadClient) Close(ctx context.Context, id string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("bead client is nil")
	}
	return c.client.Close(ctx, id)
}

func (c *trackerBeadClient) Sync(ctx context.Context) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("bead client is nil")
	}
	return c.client.Sync(ctx)
}

func (c *trackerBeadClient) AddComment(ctx context.Context, id, comment string) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("bead client is nil")
	}
	return c.client.AddComment(ctx, id, comment)
}

func (c *trackerBeadClient) GetParent(ctx context.Context, b *bead.Bead) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	if b == nil || strings.TrimSpace(b.Parent) == "" {
		return nil, nil
	}
	return c.Show(ctx, b.Parent)
}

func (c *trackerBeadClient) Create(ctx context.Context, title string, priority int, labels []string, outputs []string) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	req := buildTrackerCreateRequest(title, priority, labels, outputs, "")
	item, err := c.client.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return bead.TrackerItemToBead(item)
}

func (c *trackerBeadClient) CreateWithParent(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	req := buildTrackerCreateRequest(title, priority, labels, outputs, "")
	item, err := c.client.CreateWithParent(ctx, req, parentID)
	if err != nil {
		return nil, err
	}
	return bead.TrackerItemToBead(item)
}

func (c *trackerBeadClient) CreateWithParentAndDescription(ctx context.Context, title string, priority int, labels []string, outputs []string, parentID string, description string) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	req := buildTrackerCreateRequest(title, priority, labels, outputs, description)
	item, err := c.client.CreateWithParent(ctx, req, parentID)
	if err != nil {
		return nil, err
	}
	return bead.TrackerItemToBead(item)
}

func (c *trackerBeadClient) Update(ctx context.Context, req tracker.UpdateRequest) (*bead.Bead, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	item, err := c.client.Update(ctx, req)
	if err != nil {
		return nil, err
	}
	return bead.TrackerItemToBead(item)
}

func (c *trackerBeadClient) HasOpenChildren(ctx context.Context, parentID string) (bool, error) {
	if c == nil || c.client == nil {
		return false, fmt.Errorf("bead client is nil")
	}
	return c.client.HasOpenChildren(ctx, parentID)
}

func toBeads(items []tracker.Item) ([]*bead.Bead, error) {
	if len(items) == 0 {
		return []*bead.Bead{}, nil
	}
	result := make([]*bead.Bead, 0, len(items))
	for i := range items {
		beadObj, err := bead.TrackerItemToBead(&items[i])
		if err != nil {
			return nil, err
		}
		if beadObj != nil {
			result = append(result, beadObj)
		}
	}
	return result, nil
}

func buildTrackerCreateRequest(title string, priority int, labels, outputs []string, description string) tracker.CreateRequest {
	req := tracker.CreateRequest{
		Title:       title,
		Description: description,
		Metadata: map[string]string{
			"priority": fmt.Sprintf("%d", priority),
		},
	}
	if encoded, ok := tracker.EncodeMetadataJSONList(labels); ok {
		req.Metadata["labels"] = encoded
	}
	if encoded, ok := tracker.EncodeMetadataJSONList(outputs); ok {
		req.Metadata["expected_outputs"] = encoded
	}
	return req
}
