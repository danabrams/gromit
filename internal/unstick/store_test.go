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

func TestStoreSaveLoad(t *testing.T) {
    dir := t.TempDir()
    store := NewStore(dir)
    point := RestartPoint{Time: time.Unix(2, 0).UTC()}
    store.Set("bead-b", point)

    if err := store.Save(); err != nil {
        t.Fatalf("unexpected save error: %v", err)
    }

    reloaded := NewStore(dir)
    if err := reloaded.Load(); err != nil {
        t.Fatalf("unexpected load error: %v", err)
    }

    got, ok := reloaded.Get("bead-b")
    if !ok || !got.Time.Equal(point.Time) {
        t.Fatalf("expected loaded restart point %v, got %v (found=%v)", point.Time, got.Time, ok)
    }

    if len(reloaded.All()) != 1 {
        t.Fatalf("expected 1 restart point after load, got %d", len(reloaded.All()))
    }
}
