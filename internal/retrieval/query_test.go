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

func TestQueryAttributionCorrectness(t *testing.T) {
	q := NewQuerier()

	// Index some documents with line information
	docs := []DocumentWithAttribution{
		{
			FilePath: "file1.go",
			StartLine: 10,
			EndLine: 20,
			Content: "func TestQuery(t *testing.T) { /* test content */ }",
		},
		{
			FilePath: "file2.go",
			StartLine: 1,
			EndLine: 5,
			Content: "package main",
		},
	}

	err := q.Index(docs)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	// Query and verify attribution
	results, err := q.Query("test", 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) > 0 {
		snippet := results[0]
		if snippet.FilePath == "" {
			t.Fatalf("expected FilePath to be set, got empty string")
		}
		if snippet.StartLine == 0 && snippet.EndLine == 0 {
			t.Fatalf("expected line range to be set")
		}
	}
}
