package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	trackerpkg "github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

// QueryFromReviewBeads returns the open beads tagged with the from-review label,
// optionally scoped to a specific spec.
func QueryFromReviewBeads(ctx context.Context, tracker adapter.TaskTrackerAdapter, specFilter string) ([]*bead.Bead, error) {
	if tracker == nil {
		return nil, fmt.Errorf("task tracker adapter required for from-review")
	}
	labels := []string{"from-review"}
	if trimmed := strings.TrimSpace(specFilter); trimmed != "" {
		labels = append(labels, fmt.Sprintf("spec:%s", trimmed))
	}

	resp, err := tracker.QueryBeads(ctx, trackertypes.TaskTrackerQueryBeadsRequest{
		Labels: labels,
		Status: trackerpkg.StatusOpen,
	})
	if err != nil {
		return nil, fmt.Errorf("query from-review beads: %w", err)
	}
	if resp == nil {
		return nil, nil
	}

	beads := make([]*bead.Bead, 0, len(resp.Beads))
	for i := range resp.Beads {
		beads = append(beads, convertTrackerBead(&resp.Beads[i]))
	}
	return beads, nil
}

func convertTrackerBead(src *trackertypes.Bead) *bead.Bead {
	if src == nil {
		return nil
	}
	return &bead.Bead{
		ID:          src.ID,
		Title:       src.Title,
		Description: src.Description,
		Priority:    src.Priority,
		Labels:      cloneStrings(src.Labels),
		DependsOn:   dependencyList(src.DependsOn),
		BlockedBy:   dependencyList(src.BlockedBy),
	}
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func dependencyList(ids []string) []bead.Dependency {
	if len(ids) == 0 {
		return nil
	}
	deps := make([]bead.Dependency, 0, len(ids))
	for _, id := range ids {
		deps = append(deps, bead.Dependency{ID: id})
	}
	return deps
}
