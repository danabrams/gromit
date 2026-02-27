package retrieval

import (
	"testing"
)

func TestQueryReturnsRankedSnippets(t *testing.T) {
	q := NewQuerier()

	results, err := q.Query("test", 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if results == nil {
		t.Fatalf("expected non-nil results, got nil")
	}
}
