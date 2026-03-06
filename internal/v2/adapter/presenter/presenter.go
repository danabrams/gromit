package presenter

import (
	"context"

	"github.com/danabrams/gromit/internal/v2/presentation"
)

// PresentRequest carries the data required for a Presenter to publish a summary.
type PresentRequest struct {
	SpecID          string
	Summary         presentation.PresentationSummary
	DestinationHint string
	Metadata        map[string]string
}

// PresentResponse describes the result of a presentation attempt.
type PresentResponse struct {
	Destination  string
	Message      string
	PublishedURL string
}

// Presenter publishes spec summaries to external stakeholders.
type Presenter interface {
	Present(ctx context.Context, req PresentRequest) (PresentResponse, error)
}
