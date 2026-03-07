package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunnerPipelineDesignMentionsV2Package(t *testing.T) {
	specPath := filepath.Join("..", "..", "docs", "plans", "2026-02-21-runner-pipeline-design.md")
	body, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed to read spec document: %v", err)
	}

	if !strings.Contains(string(body), "internal/v2") {
		t.Fatalf("runner pipeline spec does not mention internal/v2 architecture")
	}
}

func TestRewritePlanMentionsRun2Command(t *testing.T) {
	planPath := filepath.Join("..", "..", "docs", "plans", "2026-02-21-rewrite-newrunnerimpl-orchestrator-wiring.md")
	body, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("failed to read plan document: %v", err)
	}

	if !strings.Contains(string(body), "run2") {
		t.Fatalf("runner rewrite plan does not mention the run2 command")
	}
}
