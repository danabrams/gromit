package pipeline

import "testing"

func TestPipelineAndPromptGofmt(t *testing.T) {
    requireGofmt(t, "internal/pipeline", "internal/prompt")
}
