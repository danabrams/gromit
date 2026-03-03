package unstick

import (
    "encoding/json"
    "os"
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

// Save writes the restart points to disk so they can be restored later.
func (s *Store) Save() error {
    s.mu.RLock()
    data := make(map[string]RestartPoint, len(s.points))
    for k, v := range s.points {
        data[k] = v
    }
    s.mu.RUnlock()

    if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
        return err
    }

    body, err := json.MarshalIndent(data, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(s.path, body, 0o644)
}

// Load populates the store from the persisted restart-point state.
func (s *Store) Load() error {
    contents, err := os.ReadFile(s.path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    var data map[string]RestartPoint
    if err := json.Unmarshal(contents, &data); err != nil {
        return err
    }

    s.mu.Lock()
    s.points = data
    s.mu.Unlock()
    return nil
}
