package pipeline

import (
	"context"
	"fmt"
	"sort"

	"github.com/danabrams/gromit/internal/bead"
)

type boardClient interface {
	ListAll(context.Context) ([]*bead.Bead, []*bead.Bead, error)
}

var newBoardClient = func() (boardClient, error) {
	return bead.NewClient()
}

// Board assembles board data with open and closed beads.
func (p *Pipeline) Board(ctx context.Context) (*BoardData, error) {
	client, err := newBoardClient()
	if err != nil {
		return nil, fmt.Errorf("creating bead client: %w", err)
	}

	open, closed, err := client.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing beads: %w", err)
	}

	sort.Slice(open, func(i, j int) bool {
		return open[i].Priority < open[j].Priority
	})

	result := NewBoardData()
	result.Open = convertToBeadInfo(open)
	result.Closed = convertToBeadInfo(closed)
	return &result, nil
}

func convertToBeadInfo(beads []*bead.Bead) []BeadInfo {
	result := make([]BeadInfo, 0, len(beads))
	for _, b := range beads {
		if b == nil {
			continue
		}
		info := BeadInfo{
			ID:       b.ID,
			Title:    b.Title,
			Priority: b.Priority,
			Labels:   append([]string(nil), b.Labels...),
		}
		result = append(result, info)
	}
	return result
}
