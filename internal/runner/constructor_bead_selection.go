package runner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/bead"
)

func buildBeadGetters(beadsClient BeadClient, labels []string) (
	func(context.Context) (*bead.Bead, error),
	func(context.Context, map[string]bool) (*bead.Bead, error),
) {
	getBead := func(ctx context.Context) (*bead.Bead, error) {
		if beadsClient == nil {
			return nil, fmt.Errorf("bead client is not configured")
		}
		if len(labels) > 0 {
			return beadsClient.ReadyWithLabel(ctx, labels[0])
		}
		return beadsClient.Ready(ctx)
	}
	getBeadExcluding := func(ctx context.Context, excludeIDs map[string]bool) (*bead.Bead, error) {
		if len(labels) > 0 {
			return getBead(ctx)
		}
		if beadsClient == nil {
			return nil, fmt.Errorf("bead client is not configured")
		}
		return beadsClient.ReadyExcluding(ctx, excludeIDs)
	}
	return getBead, getBeadExcluding
}
