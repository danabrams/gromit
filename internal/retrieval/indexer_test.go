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

func TestIndexerRefreshUpdatesMetadata(t *testing.T) {
	now := time.Date(2026, time.February, 27, 12, 0, 0, 0, time.UTC)
	calls := 0
	idx := NewIndexer(func() time.Time {
		calls++
		if calls == 1 {
			return now
		}
		return now.Add(time.Minute)
	})

	buildReq := BuildRequest{Documents: []string{"alpha"}}
	first, err := idx.Build(buildReq)
	if err != nil {
		t.Fatalf("initial build failed: %v", err)
	}

	refreshReq := RefreshRequest{
		Metadata:       first.Metadata,
		AddedDocuments: []string{"beta"},
	}

	refreshed, err := idx.Refresh(refreshReq)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}

	if refreshed.Metadata.Version != first.Metadata.Version+1 {
		t.Fatalf("expected version %d, got %d", first.Metadata.Version+1, refreshed.Metadata.Version)
	}

	if refreshed.Metadata.DocumentCount != 2 {
		t.Fatalf("expected document count 2, got %d", refreshed.Metadata.DocumentCount)
	}

	if refreshed.Metadata.LastUpdated != now.Add(time.Minute) {
		t.Fatalf("expected updated time to advance, got %v", refreshed.Metadata.LastUpdated)
	}
}
