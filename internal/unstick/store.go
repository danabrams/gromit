package unstick

import (
    "path/filepath"
    "sync"
    "time"
)

// RestartPoint records when a bead restarted.
type RestartPoint struct {
    Time time.Time
}

// Store keeps restart points.
type Store struct {
    path   string
    mu     sync.RWMutex
    points map[string]RestartPoint
}

// NewStore returns a store rooted at gromitDir.
func NewStore(gromitDir string) *Store {
    return &Store{
        path:   filepath.Join(gromitDir, ".gromit", "restart-points.json"),
        points: make(map[string]RestartPoint),
    }
}

// Set saves a restart point for beadID.
func (s *Store) Set(beadID string, point RestartPoint) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.points[beadID] = point
}

// Get retrieves a restart point if present.
func (s *Store) Get(beadID string) (RestartPoint, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    point, ok := s.points[beadID]
    return point, ok
}

// All returns the restart times for every bead.
func (s *Store) All() map[string]time.Time {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make(map[string]time.Time, len(s.points))
    for id, point := range s.points {
        result[id] = point.Time
    }
    return result
}
