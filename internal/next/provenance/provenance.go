// Package provenance tracks the lineage and freshness of workspace artifacts.
//
// Every derived artifact (architecture.json, source-map.json, guide, etc.)
// has provenance metadata recording when it was produced, from what inputs,
// and whether it is still fresh relative to the current repo state.
//
// TODO: implement provenance capture (record git SHA, timestamp, input hashes)
// TODO: implement freshness checking (compare stored SHA against HEAD)
// TODO: implement provenance-based cache invalidation
// TODO: implement provenance querying for debugging/auditing
package provenance

import "time"

// Record captures the lineage of a single artifact.
type Record struct {
	// ArtifactName identifies which artifact this record describes.
	ArtifactName string `json:"artifact_name"`

	// GitSHA is the commit hash of the repo at the time of generation.
	GitSHA string `json:"git_sha"`

	// Timestamp is when the artifact was generated.
	Timestamp time.Time `json:"timestamp"`

	// InputHashes maps input names to their content hashes.
	InputHashes map[string]string `json:"input_hashes,omitempty"`
}

// Tracker manages provenance records for a project cell.
//
// TODO: implement file-based provenance storage
// TODO: implement freshness comparison logic
type Tracker interface {
	// Record stores provenance for a newly generated artifact.
	Record(record Record) error

	// Check returns the provenance record for an artifact, if it exists.
	Check(artifactName string) (*Record, bool, error)

	// IsFresh returns true if the artifact is up-to-date with the current repo state.
	IsFresh(artifactName string, currentSHA string) (bool, error)
}
