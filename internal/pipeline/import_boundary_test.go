package pipeline

import "testing"

// TestPipelineImportBoundary ensures the pipeline packages do not reach back into
// the cmd/ layer. Any regression would mean command logic is leaking into the
// pipeline implementation.
func TestPipelineImportBoundary(t *testing.T) {
	t.Parallel()
	assertPipelinePackagesDoNotImportCmd(t)
}
