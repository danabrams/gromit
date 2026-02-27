package retrieval

import (
    "testing"
    "time"
)

func TestIndexerBuildDeterministic(t *testing.T) {
    now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
    idx := NewIndexer(func() time.Time { return now })
    req := BuildRequest{
        Documents: []string{"gamma", "alpha"},
    }

    resp1, err := idx.Build(req)
    if err != nil {
        t.Fatalf("first build failed: %v", err)
    }

    resp2, err := idx.Build(req)
    if err != nil {
        t.Fatalf("second build failed: %v", err)
    }

    if resp1.Metadata.ID != resp2.Metadata.ID {
        t.Fatalf("metadata IDs differ: %q vs %q", resp1.Metadata.ID, resp2.Metadata.ID)
    }

    if resp1.Metadata.LastUpdated != now {
        t.Fatalf("expected LastUpdated to be %v, got %v", now, resp1.Metadata.LastUpdated)
    }

    if resp1.Metadata.DocumentCount != len(req.Documents) {
        t.Fatalf("document count mismatch, want %d got %d", len(req.Documents), resp1.Metadata.DocumentCount)
    }
}
