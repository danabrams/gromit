package tui

import "time"

// RunProgressState represents run progress in a format suitable for UI rendering.
type RunProgressState struct {
	CurrentIteration int
	MaxIterations    int
	Percent          int
	Status           string
}

// ActivePhaseState represents the active phase in a format suitable for UI rendering.
type ActivePhaseState struct {
	BeadID     string
	BeadTitle  string
	Phase      string
	StartTime  time.Time
	ElapsedSec int64
}

// HealthState represents health status in a format suitable for UI rendering.
type HealthState struct {
	IsHealthy       bool
	LastEventType   string
	LastEventTime   time.Time
	HasStalledBeads bool
}

// MapRunProgress transforms the store's RunProgress into UI state.
func MapRunProgress(store *Store) *RunProgressState {
	if store == nil {
		return nil
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.Dashboard.RunProgress == nil {
		return nil
	}
	return &RunProgressState{
		CurrentIteration: store.Dashboard.RunProgress.CurrentIteration,
		MaxIterations:    store.Dashboard.RunProgress.MaxIterations,
		Percent:          store.Dashboard.RunProgress.IterationPercent,
		Status:           store.Dashboard.RunProgress.Status,
	}
}

// MapActivePhase transforms the store's ActivePhase into UI state.
func MapActivePhase(store *Store) *ActivePhaseState {
	if store == nil {
		return nil
	}
	store.mu.RLock()
	if store.Dashboard.ActivePhase == nil {
		store.mu.RUnlock()
		return nil
	}
	phase := store.Dashboard.ActivePhase
	elapsed := time.Since(phase.StartTime).Seconds()
	state := &ActivePhaseState{
		BeadID:     phase.BeadID,
		BeadTitle:  phase.BeadTitle,
		Phase:      phase.Phase,
		StartTime:  phase.StartTime,
		ElapsedSec: int64(elapsed),
	}
	store.mu.RUnlock()
	return state
}

// MapHealth transforms the store's HealthIndicator into UI state.
func MapHealth(store *Store) *HealthState {
	if store == nil {
		return &HealthState{IsHealthy: true}
	}
	store.mu.RLock()
	if store.Dashboard.HealthIndicator == nil {
		store.mu.RUnlock()
		return &HealthState{IsHealthy: true}
	}
	health := store.Dashboard.HealthIndicator
	state := &HealthState{
		IsHealthy:       health.IsHealthy,
		LastEventType:   health.LastEventType,
		LastEventTime:   health.LastEventTime,
		HasStalledBeads: health.HasStalledBeads,
	}
	store.mu.RUnlock()
	return state
}

// MapRecentCompletions transforms recent completions into a simple list.
func MapRecentCompletions(store *Store) []*Completion {
	if store == nil {
		return []*Completion{}
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if len(store.Dashboard.RecentCompletions) == 0 {
		return []*Completion{}
	}
	return append([]*Completion{}, store.Dashboard.RecentCompletions...)
}
