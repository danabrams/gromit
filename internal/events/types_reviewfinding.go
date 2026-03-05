package events

// ReviewFindingSchemaVersion is the schema version for ReviewFindingEvent instances.
const ReviewFindingSchemaVersion = 1

// ReviewFindingEvent represents a review finding emitted during the review stage.
type ReviewFindingEvent struct {
    BeadID        string   `json:"bead_id"`
    Title         string   `json:"title"`
    Description   string   `json:"description"`
    InScope       bool     `json:"in_scope"`
    AffectedFiles []string `json:"affected_files,omitempty"`
    SchemaVersion int      `json:"schema_version"`
    TimeMixin
}

// EventType returns the type identifier for ReviewFindingEvent.
func (e *ReviewFindingEvent) EventType() string {
    return "review_finding"
}
