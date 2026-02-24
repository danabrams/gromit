package pipeline

import (
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestIdeaTypeMatchesBacklogIdea(t *testing.T) {
	t.Parallel()

	var pipelineIdea *Idea
	var backlogIdea *backlog.Idea

	backlogIdea = pipelineIdea
	pipelineIdea = backlogIdea

	_ = backlogIdea
	_ = pipelineIdea
}
