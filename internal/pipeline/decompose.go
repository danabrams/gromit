package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/internal/frontmatter"
)

// Decompose executes the decompose workflow.
func (p *Pipeline) Decompose(ctx context.Context, input DecomposeInput) (*DecomposeResult, error) {
	if p.deps == nil || p.deps.ClaudeClient == nil {
		return nil, fmt.Errorf("pipeline: nil dependencies")
	}

	// Check if plan file exists
	planPath := filepath.Join(p.paths.PlansDir, input.PlanName+".md")
	if _, err := os.Stat(planPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plan not found: %s", input.PlanName)
	}

	// Read plan file frontmatter and body
	planFrontmatter, _, err := frontmatter.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}

	// Check if already decomposed (unless Force is true)
	if decomposed, ok := planFrontmatter["decomposed"].(bool); ok && decomposed && !input.Force {
		return nil, fmt.Errorf("plan already decomposed: %s", input.PlanName)
	}

	// TODO: implement full workflow
	return nil, fmt.Errorf("pipeline: Decompose not yet implemented")
}
