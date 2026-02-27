package pipeline

import (
	"context"
	"strings"
	"testing"
)

func TestPipeline_GetBoard_NilDependency(t *testing.T) {
	deps := &Deps{
		// BeadQueryClient is nil
	}
	paths := &Paths{
		GromitDir: t.TempDir(),
	}

	p := New(deps, paths)

	_, err := p.GetBoard(context.Background(), GetBoardInput{})
	if err == nil {
		t.Fatalf("GetBoard() should validate BeadQueryClient")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
