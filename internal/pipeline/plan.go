package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func runPlanWorkflow(ctx context.Context, p *Pipeline, input PlanInput) (*PlanSession, error) {
	specPath := filepath.Join(p.paths.SpecsDir, input.SpecName+".md")
	if _, err := os.Stat(specPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("spec %q not found", input.SpecName)
		}
		return nil, fmt.Errorf("checking spec %q: %w", input.SpecName, err)
	}

	return nil, fmt.Errorf("plan workflow not implemented")
}
