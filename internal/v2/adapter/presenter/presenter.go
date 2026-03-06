package presenter

import (
	"context"

	"github.com/danabrams/gromit/internal/v2/presentation"
)

// PresenterPresentRequest carries the data required for a Presenter to publish a summary.
type PresenterPresentRequest struct {
	SpecID          string
	Summary         presentation.PresentationSummary
	DestinationHint string
	Metadata        map[string]string
}

// PresentRequest is retained for backwards compatibility with existing callers.
type PresentRequest = PresenterPresentRequest

// PresenterPresentResponse describes the result of a presentation attempt.
type PresenterPresentResponse struct {
	Destination  string
	Message      string
	PublishedURL string
}

// PresentResponse is retained for backwards compatibility with existing callers.
type PresentResponse = PresenterPresentResponse

// Presenter publishes spec summaries to external stakeholders.
type Presenter interface {
	Present(ctx context.Context, req PresenterPresentRequest) (PresenterPresentResponse, error)
}
