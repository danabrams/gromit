package unstick

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"
)

func TestStoreSetGetAll(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	const reason = "initial"
	point := RestartPoint{Time: time.Unix(1, 0).UTC(), Reason: reason}
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
	got, ok = store.Get("bead-a")
	if !ok || got.Reason != reason {
		t.Fatalf("expected restart reason %q, got %q (found=%v)", reason, got.Reason, ok)
	}
}

func TestStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	const reason = "save-load"
	point := RestartPoint{Time: time.Unix(2, 0).UTC(), Reason: reason}
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
	if got.Reason != reason {
		t.Fatalf("expected loaded reason %q, got %q", reason, got.Reason)
	}

	if len(reloaded.All()) != 1 {
		t.Fatalf("expected 1 restart point after load, got %d", len(reloaded.All()))
	}
}

func TestStoreSaveAtomicPartialWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Set("bead-initial", RestartPoint{
		Time:   time.Unix(1, 0).UTC(),
		Reason: "initial",
	})
	if err := store.Save(); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	initialData, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("could not read initial store: %v", err)
	}

	store.Set("bead-next", RestartPoint{
		Time:   time.Unix(2, 0).UTC(),
		Reason: "next",
	})

	origWrite := writeFileFunc
	t.Cleanup(func() {
		writeFileFunc = origWrite
	})
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		if len(data) > 1 {
			data = data[:len(data)/2]
		}
		if err := os.WriteFile(name, data, perm); err != nil {
			return err
		}
		return errors.New("simulated partial write")
	}

	if err := store.Save(); err == nil {
		t.Fatalf("expected save error, got nil")
	}

	afterData, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("could not read store after failed save: %v", err)
	}

	if !bytes.Equal(afterData, initialData) {
		t.Fatalf("store file changed after failed save: before=%q after=%q", initialData, afterData)
	}

	tmpPath := store.path + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Fatalf("temp file %s should be removed after failed save", tmpPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking temp file: %v", err)
	}
}
