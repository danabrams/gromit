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

	if len(results) != 0 {
		t.Fatalf("expected empty results for empty index, got %d", len(results))
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

func TestQueryDeterministicRanking(t *testing.T) {
	// Create two queries with the same documents
	q1 := NewQuerier()
	q2 := NewQuerier()

	docs := []DocumentWithAttribution{
		{
			FilePath:  "alpha.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "alpha content",
		},
		{
			FilePath:  "beta.go",
			StartLine: 20,
			EndLine:   30,
			Content:   "beta content",
		},
		{
			FilePath:  "gamma.go",
			StartLine: 40,
			EndLine:   50,
			Content:   "gamma content",
		},
	}

	q1.Index(docs)
	q2.Index(docs)

	// Query both and verify results are in same order
	results1, err := q1.Query("content", 3)
	if err != nil {
		t.Fatalf("q1.Query failed: %v", err)
	}

	results2, err := q2.Query("content", 3)
	if err != nil {
		t.Fatalf("q2.Query failed: %v", err)
	}

	if len(results1) != len(results2) {
		t.Fatalf("result count mismatch: %d vs %d", len(results1), len(results2))
	}

	for i, r1 := range results1 {
		r2 := results2[i]
		if r1.FilePath != r2.FilePath {
			t.Fatalf("ranking mismatch at index %d: %q vs %q", i, r1.FilePath, r2.FilePath)
		}
		if r1.StartLine != r2.StartLine || r1.EndLine != r2.EndLine {
			t.Fatalf("line range mismatch at index %d", i)
		}
	}
}
