package retrieval

import "time"

// BuildRequest represents the documents used to construct an index.
type BuildRequest struct {
	Documents []string
}

// RefreshRequest represents incremental changes applied to an existing index.
type RefreshRequest struct {
	Metadata       IndexMetadata
	AddedDocuments []string
}

// BuildResponse exposes metadata about a completed index build.
type BuildResponse struct {
	Metadata IndexMetadata
}

// IndexMetadata captures deterministic identity information for an index build.
type IndexMetadata struct {
	ID            string
	DocumentCount int
	LastUpdated   time.Time
	Version       int
}
