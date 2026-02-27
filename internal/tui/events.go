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
	if store == nil || store.Dashboard.RunProgress == nil {
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
	if store == nil || store.Dashboard.ActivePhase == nil {
		return nil
	}
	elapsed := time.Since(store.Dashboard.ActivePhase.StartTime).Seconds()
	return &ActivePhaseState{
		BeadID:     store.Dashboard.ActivePhase.BeadID,
		BeadTitle:  store.Dashboard.ActivePhase.BeadTitle,
		Phase:      store.Dashboard.ActivePhase.Phase,
		StartTime:  store.Dashboard.ActivePhase.StartTime,
		ElapsedSec: int64(elapsed),
	}
}

// MapHealth transforms the store's HealthIndicator into UI state.
func MapHealth(store *Store) *HealthState {
	if store == nil || store.Dashboard.HealthIndicator == nil {
		return &HealthState{IsHealthy: true}
	}
	return &HealthState{
		IsHealthy:       store.Dashboard.HealthIndicator.IsHealthy,
		LastEventType:   store.Dashboard.HealthIndicator.LastEventType,
		LastEventTime:   store.Dashboard.HealthIndicator.LastEventTime,
		HasStalledBeads: store.Dashboard.HealthIndicator.HasStalledBeads,
	}
}

// MapRecentCompletions transforms recent completions into a simple list.
func MapRecentCompletions(store *Store) []*Completion {
	if store == nil || len(store.Dashboard.RecentCompletions) == 0 {
		return []*Completion{}
	}
	return store.Dashboard.RecentCompletions
}
