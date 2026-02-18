package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

func TestSpecOrchestrator_AuthorAcceptanceTests_MissingSpecReturnsError(t *testing.T) {
	renderer := &mockPromptRenderer{
		LoadSpecFn: func(name string) (string, error) {
			return "", nil
		},
	}

	router := provider.NewSingleProviderRouter(&mockProviderWithRouterTracking{})

	cfg := &config.Config{}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	orchestrator := &SpecOrchestrator{
		cfg:      cfg,
		router:   router,
		beads:    &mockBeadClient{},
		renderer: renderer,
		cmdRunnerFn: func(ctx context.Context, command string, workDir string) (string, string, int, error) {
			return "", "", 0, nil
		},
	}

	err := orchestrator.AuthorAcceptanceTests(context.Background(), "missing-spec")
	if err == nil {
		t.Fatal("expected error for missing spec, got nil")
	}
	if !strings.Contains(err.Error(), ".gromit/specs/missing-spec.md") {
		t.Fatalf("error should mention spec path, got %q", err.Error())
	}
}
