package pipeline

import (
	"reflect"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestIdeaTypeMatchesBacklogIdea(t *testing.T) {
	t.Parallel()

	pipelineType := reflect.TypeOf(Idea{})
	backlogType := reflect.TypeOf(backlog.Idea{})
	if pipelineType != backlogType {
		t.Fatalf("pipeline.Idea type = %v, want %v", pipelineType, backlogType)
	}
}
