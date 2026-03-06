package testutil

import (
	"context"
	"sync"

	"github.com/danabrams/gromit/internal/v2/presentation"
)

// PresentationCall captures a PresentSummary invocation.
type PresentationCall struct {
	SpecID  string
	Summary presentation.PresentationSummary
}

// FakePresenter records presentation summaries delivered by the code under test.
type FakePresenter struct {
	mu    sync.Mutex
	Calls []PresentationCall
}

// NewFakePresenter returns an empty presenter mock.
func NewFakePresenter() *FakePresenter {
	return &FakePresenter{}
}

// PresentSummary records the arguments for later inspection.
func (f *FakePresenter) PresentSummary(_ context.Context, specID string, summary presentation.PresentationSummary) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Calls = append(f.Calls, PresentationCall{SpecID: specID, Summary: summary})
	return nil
}
