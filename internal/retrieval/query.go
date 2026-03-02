package retrieval

import (
	"math"
	"sort"
	"strings"
)

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
	queryTokens := tokenize(query)
	docCount := len(q.documents)

	type scored struct {
		snippet Snippet
		score   float64
		index   int
	}

	if docCount == 0 {
		return []Snippet{}, nil
	}

	docTokenFreqs := make([]map[string]int, docCount)
	docFreq := map[string]int{}

	for idx, doc := range q.documents {
		freq := map[string]int{}
		seen := map[string]bool{}
		tokens := tokenize(doc.Content)

		for _, token := range tokens {
			freq[token]++
			if !seen[token] {
				docFreq[token]++
				seen[token] = true
			}
		}

		docTokenFreqs[idx] = freq
	}

	queryFreq := map[string]int{}
	for _, token := range queryTokens {
		if token == "" {
			continue
		}
		queryFreq[token]++
	}

	idf := func(token string) float64 {
		df := docFreq[token]
		return math.Log(1 + (float64(docCount)-float64(df)+0.5)/(float64(df)+0.5))
	}

	var scoredDocs []scored
	maxScore := 0.0

	for idx, doc := range q.documents {
		score := 0.0
		for token, qtf := range queryFreq {
			tf := float64(docTokenFreqs[idx][token])
			if tf == 0 {
				continue
			}
			score += idf(token) * (tf / (tf + 1)) * float64(qtf)
		}

		if score > maxScore {
			maxScore = score
		}

		scoredDocs = append(scoredDocs, scored{
			snippet: Snippet{
				Text:            doc.Content,
				FilePath:        doc.FilePath,
				StartLine:       doc.StartLine,
				EndLine:         doc.EndLine,
				ConfidenceScore: 0.0,
			},
			score: score,
			index: idx,
		})
	}

	sort.SliceStable(scoredDocs, func(i, j int) bool {
		if scoredDocs[i].score == scoredDocs[j].score {
			return scoredDocs[i].index < scoredDocs[j].index
		}
		return scoredDocs[i].score > scoredDocs[j].score
	})

	var results []Snippet
	for _, scoredDoc := range scoredDocs {
		if len(results) >= k {
			break
		}

		confidence := 0.0
		if maxScore > 0 {
			confidence = scoredDoc.score / maxScore
		}

		scoredDoc.snippet.ConfidenceScore = confidence
		results = append(results, scoredDoc.snippet)
	}

	return results, nil
}

func tokenize(text string) []string {
	var tokens []string
	for _, token := range strings.Fields(strings.ToLower(text)) {
		if token == "" {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// Snippet represents a ranked snippet with attribution.
type Snippet struct {
	Text            string
	FilePath        string
	StartLine       int
	EndLine         int
	ConfidenceScore float64
}
