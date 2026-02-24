package pipeline

import (
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestIdea_IsBacklogIdeaAlias(t *testing.T) {
	var pipelineIdea Idea
	var backlogIdea backlog.Idea

	backlogIdea = pipelineIdea
	pipelineIdea = backlogIdea
}
