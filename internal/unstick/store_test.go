package unstick

import (
	"bytes"
	"fmt"
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

func TestStoreSaveLeavesFileIntactOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	initialPoint := RestartPoint{Time: time.Unix(3, 0).UTC(), Reason: "initial"}
	store.Set("bead-c", initialPoint)
	if err := store.Save(); err != nil {
		t.Fatalf("unexpected save error: %v", err)
	}

	original, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("failed to read persisted store: %v", err)
	}

	renamed := false
	renameTempFile = func(_, _ string) error {
		renamed = true
		return fmt.Errorf("boom")
	}
	defer func() {
		renameTempFile = os.Rename
	}()

	store.Set("bead-c", RestartPoint{Time: time.Unix(4, 0).UTC(), Reason: "new"})
	if err := store.Save(); err == nil {
		t.Fatal("expected rename error")
	}
	if !renamed {
		t.Fatal("rename was not invoked")
	}

	after, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatalf("failed to read store after rename failure: %v", err)
	}
	if !bytes.Equal(original, after) {
		t.Fatalf("store mutated even though rename failed")
	}
}
