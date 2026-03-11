package enrich

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

// EnrichmentCategory identifies the type of enrichment.
type EnrichmentCategory string

const (
	CategoryComponentBoundary       EnrichmentCategory = "component_boundary"
	CategoryComponentResponsibility EnrichmentCategory = "component_responsibility"
	CategoryEntrypoint              EnrichmentCategory = "entrypoint"
	CategoryRiskyArea               EnrichmentCategory = "risky_area"
	CategoryIntegrationPoint        EnrichmentCategory = "integration_point"
	CategoryGlossaryTerm            EnrichmentCategory = "glossary_term"
	CategoryValidationSurface       EnrichmentCategory = "likely_validation_surface"
	CategoryOwnershipBoundary       EnrichmentCategory = "likely_ownership_boundary"
)

func AllCategories() []EnrichmentCategory {
	return []EnrichmentCategory{
		CategoryComponentBoundary,
		CategoryComponentResponsibility,
		CategoryEntrypoint,
		CategoryRiskyArea,
		CategoryIntegrationPoint,
		CategoryGlossaryTerm,
		CategoryValidationSurface,
		CategoryOwnershipBoundary,
	}
}

// Status tracks the review state of an inferred fact.
type Status int

const (
	StatusProposed Status = iota
	StatusAccepted
	StatusRejected
	StatusSuperseded
)

func (s Status) String() string {
	switch s {
	case StatusProposed:
		return "proposed"
	case StatusAccepted:
		return "accepted"
	case StatusRejected:
		return "rejected"
	case StatusSuperseded:
		return "superseded"
	default:
		return "unknown"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "proposed":
		*s = StatusProposed
	case "accepted":
		*s = StatusAccepted
	case "rejected":
		*s = StatusRejected
	case "superseded":
		*s = StatusSuperseded
	default:
		return fmt.Errorf("unknown status: %q", str)
	}
	return nil
}

// InferredFact represents a candidate fact proposed by an LLM.
type InferredFact struct {
	FactID         string             `json:"fact_id"`
	SourceType     string             `json:"source_type"` // always "inferred"
	Category       EnrichmentCategory `json:"category"`
	Statement      string             `json:"statement"`
	Rationale      string             `json:"rationale"`
	EvidenceRefs   []string           `json:"evidence_refs"`
	Confidence     string             `json:"confidence"`
	Scope          string             `json:"scope"`
	Status         Status             `json:"status"`
	CreatedAt      time.Time          `json:"created_at"`
	InferenceRunID string             `json:"inference_run_id"`
}

// NormalizeNilFields maps nil slices to empty values.
func (f *InferredFact) NormalizeNilFields() {
	if f.EvidenceRefs == nil {
		f.EvidenceRefs = []string{}
	}
}

// ComputeID produces a content hash from category + statement.
func (f *InferredFact) ComputeID() string {
	h := sha256.New()
	h.Write([]byte(string(f.Category)))
	h.Write([]byte{0}) // separator
	h.Write([]byte(f.Statement))
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

// EnrichInput holds the context provided to an enrichment pass.
type EnrichInput struct {
	ProjectName  string   `json:"project_name"`
	FileTree     []string `json:"file_tree"`
	Architecture string   `json:"architecture"` // JSON string of architecture artifact
	Doctrine     string   `json:"doctrine"`     // JSON string of doctrine artifact
	SourceMap    string   `json:"source_map"`   // JSON string of sourcemap artifact
	Validation   string   `json:"validation"`   // JSON string of validation artifact
	Glossary     string   `json:"glossary"`     // JSON string of glossary artifact
}
