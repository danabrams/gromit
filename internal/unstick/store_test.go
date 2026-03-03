package unstick

import (
    "testing"
    "time"
)

func TestStoreSetGetAll(t *testing.T) {
    dir := t.TempDir()
    store := NewStore(dir)

    point := RestartPoint{Time: time.Unix(1, 0).UTC()}
    store.Set("bead-a", point)

    got, ok := store.Get("bead-a")
    if !ok || !got.Time.Equal(point.Time) {
        t.Fatalf("expected stored point %v, got %v (found=%v)", point.Time, got.Time, ok)
    }

    all := store.All()
    if len(all) != 1 {
        t.Fatalf("expected 1 restart point, got %d", len(all))
    }

    storedTime, ok := all["bead-a"]
    if !ok || !storedTime.Equal(point.Time) {
        t.Fatalf("expected restart point time %v, got %v (found=%v)", point.Time, storedTime, ok)
    }
}
