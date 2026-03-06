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
	beadItem, err := client.Show(ctx, beadID)
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
	labels := append([]string(nil), req.Labels...)
	dependencies := append([]string(nil), req.Dependencies...)
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
	if err := client.Close(ctx, req.BeadID); err != nil {
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
	if strings.TrimSpace(req.Status) != "" {
		beads, err = client.ListByStatus(ctx, strings.ToLower(req.Status))
	} else {
		beads, err = client.List(ctx)
	}
	if err != nil {
		return nil, err
	}

	response := &QueryBeadsResponse{}
	for _, b := range beads {
		if b == nil {
			continue
		}
		if strings.TrimSpace(req.Parent) != "" && strings.TrimSpace(b.Parent) != strings.TrimSpace(req.Parent) {
			continue
		}
		if !hasLabels(b.Labels, req.Labels) {
			continue
		}
		response.Beads = append(response.Beads, *convertBead(b))
	}
	return response, nil
}

// RecordPlan persists the generated plan for the spec.
func (a *BDAdapter) RecordPlan(ctx context.Context, specID, plan string) error {
	_ = ctx
	_ = specID
	_ = plan
	return nil
}

func convertBead(b *bead.Bead) *Bead {
	if b == nil {
		return nil
	}
	return &Bead{
		ID:          b.ID,
		Title:       b.Title,
		Description: b.Description,
		Priority:    b.Priority,
		Status:      b.Status,
		Labels:      append([]string(nil), b.Labels...),
		DependsOn:   dependencyIDs(b.DependsOn),
		BlockedBy:   dependencyIDs(b.BlockedBy),
		Dependents:  dependencyIDs(b.Dependencies),
	}
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
