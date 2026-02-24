package pipeline

import (
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestIdeaTypeIsUnifiedWithBacklogIdea(t *testing.T) {
	t.Parallel()

	var pipelineIdea Idea
	var backlogIdea backlog.Idea

	backlogIdea = pipelineIdea
	pipelineIdea = backlogIdea
}
