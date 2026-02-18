package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/provider"
)

// SpecOrchestrator coordinates spec-level acceptance test authoring.
type SpecOrchestrator struct {
	renderer    PromptRenderer
	router      *provider.Router
	beads       BeadClient
	cfg         *config.Config
	cmdRunnerFn func(ctx context.Context, command string, workDir string) (string, string, int, error)

	authoredSpecs map[string]bool
}

// AuthorAcceptanceTests loads the spec, renders the acceptance prompt, and invokes the provider.
func (o *SpecOrchestrator) AuthorAcceptanceTests(ctx context.Context, specName string) error {
	if o == nil {
		return fmt.Errorf("spec orchestrator is nil")
	}
	if o.renderer == nil {
		return fmt.Errorf("spec orchestrator renderer is nil")
	}
	if o.router == nil {
		return fmt.Errorf("spec orchestrator router is nil")
	}
	if specName == "" {
		return fmt.Errorf("spec name is empty")
	}

	if o.authoredSpecs != nil && o.authoredSpecs[specName] {
		return nil
	}

	spec, err := o.renderer.LoadSpec(specName)
	if err != nil {
		return fmt.Errorf("loading spec %s: %w", specName, err)
	}
	if strings.TrimSpace(spec) == "" {
		return fmt.Errorf("spec file .gromit/specs/%s.md not found", specName)
	}

	if o.authoredSpecs == nil {
		o.authoredSpecs = make(map[string]bool)
	}
	o.authoredSpecs[specName] = true
	return nil
}
