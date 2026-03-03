package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
)

type eventEmitter interface {
	Emit(events.Event)
}

// ListStuck returns beads that are currently marked as stuck according to queue logic.
func (p *Pipeline) ListStuck(ctx context.Context, input QueueInput) ([]*bead.Bead, error) {
	result, err := p.Queue(ctx, input)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return []*bead.Bead{}, nil
	}
	if result.Stuck == nil {
		return []*bead.Bead{}, nil
	}
	return result.Stuck, nil
}

// WithEmitter attaches an event emitter to the pipeline for manual status paths such as unstick.
func (p *Pipeline) WithEmitter(emitter eventEmitter) *Pipeline {
	if p == nil {
		return nil
	}
	p.emitter = emitter
	return p
}

// Unstick marks a bead as unstuck by writing a restart point and emitting the corresponding event.
func (p *Pipeline) Unstick(ctx context.Context, beadID, gromitDir string) error {
	beadID = strings.TrimSpace(beadID)
	if beadID == "" {
		return fmt.Errorf("bead id is required")
	}

	client, err := p.queueClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	active, err := ListActiveBeads(ctx, client)
	if err != nil {
		return fmt.Errorf("listing active beads: %w", err)
	}

	found := false
	for _, b := range active {
		if b != nil && b.ID == beadID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("bead %s not found", beadID)
	}

	if err := writeManualRestartPoint(gromitDir, beadID); err != nil {
		return err
	}

	if p.emitter != nil {
		p.emitter.Emit(&events.BeadUnstickedEvent{
			BeadID: beadID,
			Reason: "manual",
		})
	}

	return nil
}

func writeManualRestartPoint(gromitDir, beadID string) error {
	store := newRestartPointStore(gromitDir)
	if err := store.load(); err != nil {
		return fmt.Errorf("loading restart points: %w", err)
	}
	store.points[beadID] = restartPoint{
		RestartAt: time.Now().UTC(),
		Reason:    "manual",
	}
	if err := store.save(); err != nil {
		return fmt.Errorf("writing restart point: %w", err)
	}
	return nil
}

type restartPoint struct {
	RestartAt time.Time `json:"restart_at"`
	Reason    string    `json:"reason,omitempty"`
}

type restartPointStore struct {
	path   string
	points map[string]restartPoint
}

func newRestartPointStore(gromitDir string) *restartPointStore {
	return &restartPointStore{
		path:   filepath.Join(gromitDir, "restart-points.json"),
		points: map[string]restartPoint{},
	}
}

func (s *restartPointStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.points = map[string]restartPoint{}
			return nil
		}
		return err
	}
	if len(data) == 0 {
		s.points = map[string]restartPoint{}
		return nil
	}

	points := map[string]restartPoint{}
	if err := json.Unmarshal(data, &points); err != nil {
		return err
	}
	s.points = points
	return nil
}

func (s *restartPointStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating restart points directory: %w", err)
	}

	data, err := json.MarshalIndent(s.points, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling restart points: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("writing restart points: %w", err)
	}

	return nil
}
