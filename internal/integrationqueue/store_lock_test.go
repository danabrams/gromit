package integrationqueue

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestStoreLoadBlocksWhileQueueLocked(t *testing.T) {
	dir := t.TempDir()

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	lockFile := openQueueLockFileForTesting(t, store.path)
	t.Cleanup(func() {
		_ = lockFile.Close()
	})

	lockQueueFile(t, lockFile)

	done := make(chan struct{})
	go func() {
		_, err := store.load()
		if err != nil {
			t.Errorf("store.load error: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		t.Fatalf("store.load completed while queue lock was still held")
	case <-time.After(50 * time.Millisecond):
	}

	unlockQueueFile(t, lockFile)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("store.load did not finish after top-level lock was released")
	}
}

func openQueueLockFileForTesting(t *testing.T, queuePath string) *os.File {
	t.Helper()

	lockPath := queuePath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating lock directory: %v", err)
	}

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("opening lock file: %v", err)
	}
	return file
}

func lockQueueFile(t *testing.T, file *os.File) {
	t.Helper()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("locking queue file: %v", err)
	}
}

func unlockQueueFile(t *testing.T, file *os.File) {
	t.Helper()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("unlocking queue file: %v", err)
	}
}
