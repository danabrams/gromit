package pipeline

import (
	"context"
	"strings"
	"testing"
)

func TestPipeline_GetQueue_NilDependency(t *testing.T) {
	deps := &Deps{
		// BeadQueryClient is nil
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	_, err := p.GetQueue(context.Background(), GetQueueInput{})
	if err == nil {
		t.Fatalf("GetQueue() should validate BeadQueryClient")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
