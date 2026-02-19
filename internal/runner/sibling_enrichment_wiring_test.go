package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/prompt"
)

func TestWireSiblingEnrichmentResolver_SetsResolverAndUsesLoggerHelper(t *testing.T) {
	logsDir := t.TempDir()
	logPath := filepath.Join(logsDir, "run-test.jsonl")
	content := "" +
		"{\"iteration\":1,\"bead_id\":\"self\",\"success\":true,\"spec_id\":\"spec-a\",\"touched_packages\":[\"internal/self\"]}\n" +
		"{\"iteration\":1,\"bead_id\":\"sib-1\",\"success\":true,\"spec_id\":\"spec-a\",\"touched_packages\":[\"internal/a\",\"internal/shared\"]}\n" +
		"{\"iteration\":1,\"bead_id\":\"sib-2\",\"success\":false,\"spec_id\":\"spec-a\",\"touched_packages\":[\"internal/failed\"]}\n" +
		"{\"iteration\":1,\"bead_id\":\"other\",\"success\":true,\"spec_id\":\"spec-b\",\"touched_packages\":[\"internal/other\"]}\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	var gotResolver prompt.SiblingTouchedPackagesResolver
	mockRenderer := &mockPromptRenderer{
		SetSiblingResolverFn: func(resolver prompt.SiblingTouchedPackagesResolver) {
			gotResolver = resolver
		},
	}

	wireSiblingEnrichmentResolver(mockRenderer, logsDir)
	if gotResolver == nil {
		t.Fatal("expected sibling resolver to be wired")
	}

	pkgs, err := gotResolver(&bead.Bead{
		ID:              "self",
		Labels:          []string{"spec:spec-a"},
		ExpectedOutputs: []string{},
	}, nil)
	if err != nil {
		t.Fatalf("resolver returned error: %v", err)
	}
	if len(pkgs) != 2 || pkgs[0] != "internal/a" || pkgs[1] != "internal/shared" {
		t.Fatalf("resolver packages = %v, want [internal/a internal/shared]", pkgs)
	}
}
