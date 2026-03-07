package tasktracker

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
)

// BDAdapter adapts the bd CLI to provide TaskTracker functionality
type BDAdapter struct {
	client *bead.Client
}

// NewBDAdapter creates a new BDAdapter backed by the provided bd client.
func NewBDAdapter(client *bead.Client) *BDAdapter {
	return &BDAdapter{client: client}
}

func (a *BDAdapter) clientOrErr() (*bead.Client, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("bd adapter client is nil")
	}
	return a.client, nil
}

// NextBead returns the next open bead with dependency information.
func (a *BDAdapter) NextBead(ctx context.Context, req NextBeadRequest) (*NextBeadResponse, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	beadItem, err := client.Ready(ctx)
	if err != nil {
		return nil, err
	}
	return &NextBeadResponse{Bead: convertBead(beadItem)}, nil
}

// ShowBead returns metadata for the requested bead ID.
func (a *BDAdapter) ShowBead(ctx context.Context, beadID string) (*Bead, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	beadItem, err := client.Show(ctx, normalizeString(beadID))
	if err != nil {
		return nil, err
	}
	return convertBead(beadItem), nil
}

// CreateBead creates a new bead with the given properties via bd.
func (a *BDAdapter) CreateBead(ctx context.Context, req CreateBeadRequest) (*CreateBeadResponse, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	labels := copyStrings(req.Labels)
	dependencies := copyStrings(req.Dependencies)
	beadItem, err := client.CreateWithDepsAndDescription(ctx, req.Title, req.Priority, labels, nil, dependencies, req.Description)
	if err != nil {
		return nil, err
	}
	return &CreateBeadResponse{Bead: convertBead(beadItem)}, nil
}

// CloseBead marks a bead as closed via bd.
func (a *BDAdapter) CloseBead(ctx context.Context, req CloseBeadRequest) (*CloseBeadResponse, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}
	id := normalizeString(req.BeadID)
	if id == "" {
		return nil, fmt.Errorf("bead ID is required")
	}
	if err := client.Close(ctx, id); err != nil {
		return nil, err
	}
	return &CloseBeadResponse{Closed: true}, nil
}

// QueryBeads filters beads by labels, status, and parent in bd.
func (a *BDAdapter) QueryBeads(ctx context.Context, req QueryBeadsRequest) (*QueryBeadsResponse, error) {
	client, err := a.clientOrErr()
	if err != nil {
		return nil, err
	}

	var beads []*bead.Bead
	status := normalizeString(req.Status)
	if status != "" {
		beads, err = client.ListByStatus(ctx, strings.ToLower(status))
	} else {
		beads, err = client.List(ctx)
	}
	if err != nil {
		return nil, err
	}

	response := &QueryBeadsResponse{}
	parent := normalizeString(req.Parent)
	for _, b := range beads {
		if b == nil {
			continue
		}
		if parent != "" && normalizeString(b.Parent) != parent {
			continue
		}
		if !hasLabels(b.Labels, req.Labels) {
			continue
		}
		response.Beads = append(response.Beads, *convertBead(b))
	}
	return response, nil
}

func convertBead(b *bead.Bead) *Bead {
	if b == nil {
		return nil
	}
	blockers, dependents := splitDependenciesByType(b.Dependencies)
	return &Bead{
		ID:          b.ID,
		Title:       b.Title,
		Description: b.Description,
		Priority:    b.Priority,
		Status:      b.Status,
		Labels:      copyStrings(b.Labels),
		DependsOn:   dependencyIDs(b.DependsOn),
		BlockedBy:   appendIDs(dependencyIDs(b.BlockedBy), blockers),
		Dependents:  dependents,
	}
}

// splitDependenciesByType partitions b.Dependencies into blockers (entries with
// dependency_type "blocks") and dependents (everything else).
func splitDependenciesByType(deps []bead.Dependency) (blockers, dependents []string) {
	for _, d := range deps {
		id := strings.TrimSpace(d.ID)
		if id == "" {
			continue
		}
		if d.DependencyType == "blocks" {
			blockers = append(blockers, id)
		} else {
			dependents = append(dependents, id)
		}
	}
	return blockers, dependents
}

// appendIDs appends additional IDs to an existing slice, returning the combined result.
func appendIDs(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
	if len(base) == 0 {
		return extra
	}
	return append(base, extra...)
}

func dependencyIDs(deps []bead.Dependency) []string {
	if len(deps) == 0 {
		return nil
	}
	ids := make([]string, 0, len(deps))
	for _, dep := range deps {
		if trimmed := strings.TrimSpace(dep.ID); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

func hasLabels(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	for _, needed := range want {
		needed = strings.TrimSpace(needed)
		if needed == "" {
			continue
		}
		found := false
		for _, label := range have {
			if label == needed {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func copyStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func normalizeString(value string) string {
	return strings.TrimSpace(value)
}
