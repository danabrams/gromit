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
			FilePath:  "file1.go",
			StartLine: 10,
			EndLine:   20,
			Content:   "func TestQuery(t *testing.T) { /* test content */ }",
		},
		{
			FilePath:  "file2.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "package main",
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

func TestQueryConfidenceBoundedness(t *testing.T) {
	q := NewQuerier()

	docs := []DocumentWithAttribution{
		{
			FilePath:  "test.go",
			StartLine: 1,
			EndLine:   10,
			Content:   "test content",
		},
	}

	q.Index(docs)

	results, err := q.Query("test", 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	for i, snippet := range results {
		if snippet.ConfidenceScore < 0.0 || snippet.ConfidenceScore > 1.0 {
			t.Fatalf("confidence score at index %d out of bounds: %f", i, snippet.ConfidenceScore)
		}
	}
}

func TestQueryRespectsKLimit(t *testing.T) {
	q := NewQuerier()

	// Index 10 documents
	docs := make([]DocumentWithAttribution, 10)
	for i := 0; i < 10; i++ {
		docs[i] = DocumentWithAttribution{
			FilePath:  "file.go",
			StartLine: i * 10,
			EndLine:   i*10 + 5,
			Content:   "content",
		}
	}

	q.Index(docs)

	// Query with K=3
	results, err := q.Query("content", 3)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Query with K=20 (more than available)
	results, err = q.Query("content", 20)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 results (all available), got %d", len(results))
	}
}

func TestQueryAttributionIncludesExactLineRanges(t *testing.T) {
	q := NewQuerier()

	// Index a document with specific line range
	docs := []DocumentWithAttribution{
		{
			FilePath:  "handler.go",
			StartLine: 42,
			EndLine:   67,
			Content:   "func HandleRequest(w http.ResponseWriter, r *http.Request) { /* implementation */ }",
		},
	}

	q.Index(docs)

	results, err := q.Query("handler", 5)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatalf("expected results, got none")
	}

	snippet := results[0]
	if snippet.FilePath != "handler.go" {
		t.Fatalf("expected FilePath 'handler.go', got %q", snippet.FilePath)
	}

	if snippet.StartLine != 42 {
		t.Fatalf("expected StartLine 42, got %d", snippet.StartLine)
	}

	if snippet.EndLine != 67 {
		t.Fatalf("expected EndLine 67, got %d", snippet.EndLine)
	}
}

func TestQueryRanksByRelevance(t *testing.T) {
	q := NewQuerier()

	docs := []DocumentWithAttribution{
		{
			FilePath:  "relevant.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "topic query topic keyword",
		},
		{
			FilePath:  "less_relevant.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "topic keyword",
		},
		{
			FilePath:  "irrelevant.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "unrelated content",
		},
	}

	if err := q.Index(docs); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	results, err := q.Query("topic keyword", 3)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].FilePath != "relevant.go" {
		t.Fatalf("expected most relevant doc first, got %s", results[0].FilePath)
	}

	if !(results[0].ConfidenceScore > results[1].ConfidenceScore) {
		t.Fatalf("expected first confidence > second, got %f vs %f", results[0].ConfidenceScore, results[1].ConfidenceScore)
	}

	if !(results[1].ConfidenceScore > results[2].ConfidenceScore) {
		t.Fatalf("expected second confidence > third, got %f vs %f", results[1].ConfidenceScore, results[2].ConfidenceScore)
	}
}

func TestQueryPrioritizesRareTerms(t *testing.T) {
	q := NewQuerier()

	docs := []DocumentWithAttribution{
		{
			FilePath:  "common.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "common common common common",
		},
		{
			FilePath:  "rare.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "rare",
		},
		{
			FilePath:  "common_shared.go",
			StartLine: 1,
			EndLine:   5,
			Content:   "common filler",
		},
	}

	if err := q.Index(docs); err != nil {
		t.Fatalf("Index failed: %v", err)
	}

	results, err := q.Query("common rare", 2)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if results[0].FilePath != "rare.go" {
		t.Fatalf("expected rare term doc first, got %s", results[0].FilePath)
	}

	if !(results[0].ConfidenceScore > results[1].ConfidenceScore) {
		t.Fatalf("expected first confidence > second, got %f vs %f", results[0].ConfidenceScore, results[1].ConfidenceScore)
	}
}
