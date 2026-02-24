package pipeline

import (
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestIdeaIsCanonicalBacklogType(t *testing.T) {
	t.Parallel()

	var backlogIdea *backlog.Idea
	var pipelineIdea *Idea

	pipelineIdea = backlogIdea
	if pipelineIdea != nil {
		t.Fatalf("expected nil pointer assignment to remain nil")
	}
}
