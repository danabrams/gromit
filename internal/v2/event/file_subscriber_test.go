package event

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileSubscriberStoresPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	fs := NewFileSubscriber(path)
	if fs.path != path {
		t.Fatalf("expected path %q, got %q", path, fs.path)
	}
}
