package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/danabrams/gromit/skills"
)

func runPlanWorkflow(ctx context.Context, p *Pipeline, input PlanInput) (*PlanSession, error) {
	// Validate dependencies
	if err := validateRequiredDeps([]namedDependency{
		{name: "PlanRenderer", dep: p.deps.PlanRenderer},
		{name: "AgentResolver", dep: p.deps.AgentResolver},
	}); err != nil {
		return nil, err
	}

	// Load spec file
	specPath := filepath.Join(p.paths.SpecsDir, input.SpecName+".md")
	if _, err := os.Stat(specPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("spec %q not found", input.SpecName)
		}
		return nil, fmt.Errorf("checking spec %q: %w", input.SpecName, err)
	}

	// Check for duplicate plan (unless Force=true)
	if !input.Force {
		planPath := filepath.Join(p.paths.PlansDir, input.SpecName+".md")
		if _, err := os.Stat(planPath); err == nil {
			return nil, fmt.Errorf("plan for spec %q already exists", input.SpecName)
		}
	}

	// Read spec content
	specFile, err := os.Open(specPath)
	if err != nil {
		return nil, fmt.Errorf("opening spec file: %w", err)
	}
	defer specFile.Close()

	specContent, err := io.ReadAll(specFile)
	if err != nil {
		return nil, fmt.Errorf("reading spec file: %w", err)
	}

	// Assemble open-bead context (minimal context for now)
	openBeadContext := fmt.Sprintf("Planning context for %s spec.", input.SpecName)

	// Create prompt input with spec content and context
	promptInput := &PlanPromptInput{
		IdeaText: fmt.Sprintf("%s\n\n%s\n\nContext: %s", string(specContent), skills.PlanSkill, openBeadContext),
	}

	// Render prompt
	renderedPrompt, err := p.deps.PlanRenderer.RenderPlan(promptInput)
	if err != nil {
		return nil, fmt.Errorf("rendering plan prompt: %w", err)
	}

	// Write temp file
	tmpDir := filepath.Join(p.paths.GromitDir, "tmp")
	promptPath, cleanup, err := writeTempPromptWithPattern(tmpDir, "plan-prompt-*.md", renderedPrompt)
	if err != nil {
		return nil, err
	}

	// Resolve agent
	agent, err := p.deps.AgentResolver.Resolve("plan", input.AgentName, input.ChooseAgent)
	if err != nil {
		cleanup() // Clean up on error before returning
		return nil, fmt.Errorf("resolving agent: %w", err)
	}

	// Launch agent
	if err := agent.LaunchInDir(promptPath, input.LaunchDir); err != nil {
		cleanup() // Clean up on error before returning
		return nil, fmt.Errorf("launching agent: %w", err)
	}

	// Clean up the temp file now that agent has launched
	cleanup()

	// Return session that owns the cleanup function
	// Caller must call session.Cleanup() to remove the temp file
	return NewPlanSession(cleanup), nil
}
