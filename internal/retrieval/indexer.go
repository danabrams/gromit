package retrieval

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"time"
)

// Indexer manages the lifecycle of retrieval indexes.
type Indexer struct {
	now func() time.Time
}

// NewIndexer returns an Indexer that uses the provided clock for determinism.
func NewIndexer(now func() time.Time) *Indexer {
	if now == nil {
		now = time.Now
	}
	return &Indexer{now: now}
}

// Build creates a new index metadata snapshot from the request.
func (i *Indexer) Build(req BuildRequest) (BuildResponse, error) {
	id := computeID(req.Documents)
	metadata := IndexMetadata{
		ID:            id,
		DocumentCount: len(req.Documents),
		LastUpdated:   i.now(),
		Version:       1,
	}
	return BuildResponse{Metadata: metadata}, nil
}

// Refresh applies incremental updates to an existing index snapshot.
func (i *Indexer) Refresh(req RefreshRequest) (BuildResponse, error) {
	metadata := req.Metadata
	metadata.DocumentCount += len(req.AddedDocuments)
	metadata.Version++
	metadata.LastUpdated = i.now()
	return BuildResponse{Metadata: metadata}, nil
}

func computeID(documents []string) string {
	sorted := append([]string(nil), documents...)
	sort.Strings(sorted)
	hasher := sha256.New()
	for _, doc := range sorted {
		hasher.Write([]byte(doc))
		hasher.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
