package retrieval

// Querier handles top-K queries over indexed documents with attribution.
type Querier struct {
	documents []DocumentWithAttribution
}

// NewQuerier creates a new Querier instance.
func NewQuerier() *Querier {
	return &Querier{
		documents: []DocumentWithAttribution{},
	}
}

// DocumentWithAttribution represents a document segment with precise attribution.
type DocumentWithAttribution struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
}

// Index adds documents to the querier's index.
func (q *Querier) Index(docs []DocumentWithAttribution) error {
	q.documents = append(q.documents, docs...)
	return nil
}

// Query performs a top-K retrieval query.
func (q *Querier) Query(query string, k int) ([]Snippet, error) {
	var results []Snippet

	// Return indexed documents as snippets
	for _, doc := range q.documents {
		snippet := Snippet{
			Text:            doc.Content,
			FilePath:        doc.FilePath,
			StartLine:       doc.StartLine,
			EndLine:         doc.EndLine,
			ConfidenceScore: 1.0,
		}
		results = append(results, snippet)
		if len(results) >= k {
			break
		}
	}

	return results, nil
}

// Snippet represents a ranked snippet with attribution.
type Snippet struct {
	Text            string
	FilePath        string
	StartLine       int
	EndLine         int
	ConfidenceScore float64
}
