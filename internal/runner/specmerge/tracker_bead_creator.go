package specmerge

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/danabrams/gromit/internal/specgate"
	"github.com/danabrams/gromit/internal/tracker"
)

// NewTrackerBeadCreator wraps a tracker.Client to satisfy specgate.BeadCreator.
func NewTrackerBeadCreator(client tracker.Client) specgate.BeadCreator {
	if client == nil {
		return nil
	}
	return &trackerBeadCreator{trackerClient: client}
}

type trackerBeadCreator struct {
	trackerClient tracker.Client
}

func (c *trackerBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if c == nil || c.trackerClient == nil {
		return "", fmt.Errorf("tracker bead creator is not configured")
	}

	priorityInt, err := parsePriorityLabel(priority)
	if err != nil {
		return "", err
	}

	metadata := make(map[string]string)
	metadata["priority"] = strconv.Itoa(priorityInt)
	if encoded, ok := tracker.EncodeMetadataJSONList(labels); ok {
		metadata["labels"] = encoded
	}

	req := tracker.CreateRequest{
		Title:       title,
		Description: description,
		Metadata:    metadata,
	}

	item, err := c.trackerClient.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if item == nil {
		return "", fmt.Errorf("tracker.Create returned nil item")
	}
	return item.ID, nil
}

func parsePriorityLabel(label string) (int, error) {
	trimmed := strings.TrimSpace(label)
	if trimmed == "" {
		return 0, fmt.Errorf("priority is empty")
	}
	if strings.HasPrefix(strings.ToUpper(trimmed), "P") {
		trimmed = strings.TrimSpace(trimmed[1:])
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid priority %q", label)
	}
	return value, nil
}
