package retrieval

import "time"

// BuildRequest represents the documents used to construct an index.
type BuildRequest struct {
	Documents []string
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
