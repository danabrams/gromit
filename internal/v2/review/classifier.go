package review

import (
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/events"
	reviewpkg "github.com/danabrams/gromit/internal/review"
	"github.com/danabrams/gromit/internal/v2/generation"
)

// Finding represents a review observation that may or may not belong to the current scope.
type Finding struct {
	Title         string
	Description   string
	Priority      int
	AffectedFiles []string
	InScope       bool
}

// ClassificationResult captures how review findings were handled.
type ClassificationResult struct {
	Beads      []*bead.Bead
	OutOfScope []Finding
}

// Classifier applies review findings to the run loop (beads and events).
type Classifier struct {
	emitter *events.Emitter
}

// NewClassifier returns a classifier optionally backed by an event emitter.
func NewClassifier(emitter *events.Emitter) *Classifier {
	return &Classifier{emitter: emitter}
}

// Classify processes findings and produces beads for those that are in scope.
func (c *Classifier) Classify(parent *bead.Bead, findings []Finding) ClassificationResult {
	result := ClassificationResult{}
	parentGen := 0
	if parent != nil {
		parentGen = generation.Current(parent.Labels)
	}
	nextGenLabel := generation.Format(parentGen + 1)
	parentID := ""
	if parent != nil {
		parentID = parent.ID
	}

	for _, finding := range findings {
		c.emitReviewFinding(parentID, finding)
		if finding.InScope {
			labels := reviewpkg.BuildReviewBeadLabels([]string{nextGenLabel})
			newBead := &bead.Bead{
				Title:       finding.Title,
				Description: finding.Description,
				Priority:    resolvePriority(finding.Priority),
				Labels:      labels,
			}
			result.Beads = append(result.Beads, newBead)
		} else {
			result.OutOfScope = append(result.OutOfScope, finding)
		}
	}

	return result
}

func (c *Classifier) emitReviewFinding(beadID string, finding Finding) {
	if c == nil || c.emitter == nil {
		return
	}
	c.emitter.Emit(&events.ReviewFindingEvent{
		BeadID:        beadID,
		Title:         finding.Title,
		Description:   finding.Description,
		InScope:       finding.InScope,
		AffectedFiles: append([]string(nil), finding.AffectedFiles...),
		SchemaVersion: events.ReviewFindingSchemaVersion,
	})
}

func resolvePriority(requested int) int {
	if requested <= 0 {
		return 1
	}
	if requested > 4 {
		return 4
	}
	return requested
}
