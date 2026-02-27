package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
)

// AddInput defines parameters for adding a backlog idea.
type AddInput struct {
	Text    string
	Context string
	Type    string
}

// AddResult reports the outcome of creating a backlog idea.
type AddResult struct {
	Idea *Idea
	Type string
}

// ErrUnknownIdeaType is returned when the idea type cannot be determined automatically.
var ErrUnknownIdeaType = errors.New("pipeline: unknown idea type")

// Add creates a backlog idea when the type is known or auto-classified.
func (p *Pipeline) Add(ctx context.Context, input AddInput) (*AddResult, error) {
	_ = ctx

	if p == nil || p.deps == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}
	if err := requireNonNilDep("BacklogClient", p.deps.BacklogClient); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(input.Text)
	if text == "" {
		return nil, fmt.Errorf("pipeline: idea text is required")
	}

	ideaType := determineIdeaTypeFromInput(input.Type, text)
	if ideaType == "unknown" {
		return nil, ErrUnknownIdeaType
	}

	idea := &Idea{
		ID:        backlog.GenerateID(),
		Text:      text,
		Type:      ideaType,
		Context:   strings.TrimSpace(input.Context),
		CreatedAt: time.Now(),
	}

	if err := p.deps.BacklogClient.Add(idea); err != nil {
		return nil, fmt.Errorf("adding backlog idea: %w", err)
	}

	return &AddResult{Idea: idea, Type: ideaType}, nil
}

func determineIdeaTypeFromInput(typeHint, text string) string {
	normalized := normalizeIdeaType(typeHint)
	if normalized == "" || normalized == "unknown" {
		return categorizeByText(text)
	}
	return normalized
}
