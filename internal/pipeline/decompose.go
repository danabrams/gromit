package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/skills"
)

// beadDef represents a bead definition from Claude's JSON output.
type beadDef struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Priority           string   `json:"priority"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	DependsOnIndex     []int    `json:"depends_on_index"`
}

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
	planFrontmatter, planBody, err := frontmatter.ReadFile(planPath)
	if err != nil {
		return nil, fmt.Errorf("reading plan file: %w", err)
	}

	// Check if already decomposed (unless Force is true)
	if decomposed, ok := planFrontmatter["decomposed"].(bool); ok && decomposed && !input.Force {
		return nil, fmt.Errorf("plan already decomposed: %s", input.PlanName)
	}

	// Build prompt with embedded skill content
	prompt := buildDecomposePrompt(input.PlanName, planBody, skills.DecomposeSkill)

	// Run Claude non-interactively
	claudeResult, err := p.deps.ClaudeClient.Run(prompt, "sonnet")
	if err != nil {
		return nil, fmt.Errorf("invoking Claude: %w", err)
	}

	// Check if the result has Success field
	resultMap, ok := claudeResult.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected Claude result type")
	}

	success, _ := resultMap["Success"].(bool)
	if !success {
		exitCode, _ := resultMap["ExitCode"].(int)
		output, _ := resultMap["Output"].(string)
		return nil, fmt.Errorf("Claude invocation failed (exit code %d)\nOutput:\n%s", exitCode, output)
	}

	// Parse JSON array from result
	output, _ := resultMap["Output"].(string)
	var beadDefs []beadDef
	if err := jsonutil.ExtractJSON(output, &beadDefs); err != nil {
		return nil, fmt.Errorf("parsing bead definitions: %w", err)
	}

	if len(beadDefs) == 0 {
		return nil, fmt.Errorf("no beads extracted from plan")
	}

	// Review mode: return proposed beads without creating them
	if input.Review {
		result := NewDecomposeResult()
		for _, def := range beadDefs {
			priority := parsePriority(def.Priority)
			labels := []string{fmt.Sprintf("spec:%s", input.PlanName)}

			bead := NewCreatedBead()
			bead.ID = "" // Not created yet
			bead.Title = def.Title
			bead.Priority = priority
			bead.Labels = labels
			result.CreatedBeads = append(result.CreatedBeads, bead)
		}
		result.PlanUpdated = false
		return &result, nil
	}

	// Create beads
	result := NewDecomposeResult()
	createdIDs := []string{}

	for i, def := range beadDefs {
		// Map priority string to int
		priority := parsePriority(def.Priority)

		// Build labels: always include spec:<name>
		labels := []string{fmt.Sprintf("spec:%s", input.PlanName)}

		// Resolve dependencies from depends_on_index
		var dependencies []string
		for _, depIdx := range def.DependsOnIndex {
			// Skip self-dependencies
			if depIdx == i {
				continue
			}
			// Skip out-of-range dependencies
			if depIdx < 0 || depIdx >= len(createdIDs) {
				continue
			}
			dependencies = append(dependencies, createdIDs[depIdx])
		}

		// Create bead via BeadClient
		beadResult, err := p.deps.BeadClient.CreateWithDepsAndDescription(
			def.Title,
			priority,
			labels,
			def.AcceptanceCriteria,
			dependencies,
			def.Description,
		)
		if err != nil {
			return nil, fmt.Errorf("creating bead %d: %w", i, err)
		}

		// Extract ID from result
		beadMap, ok := beadResult.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected bead result type")
		}
		beadID, ok := beadMap["ID"].(string)
		if !ok {
			return nil, fmt.Errorf("bead result missing ID field")
		}

		createdIDs = append(createdIDs, beadID)

		// Add to result
		bead := NewCreatedBead()
		bead.ID = beadID
		bead.Title = def.Title
		bead.Priority = priority
		bead.Labels = labels
		result.CreatedBeads = append(result.CreatedBeads, bead)
	}

	// TODO: update plan frontmatter
	result.PlanUpdated = false
	return &result, nil
}

// buildDecomposePrompt constructs the prompt for Claude.
func buildDecomposePrompt(planName, planBody, skillContent string) string {
	return fmt.Sprintf(`# Decompose Plan: %s

You are decomposing an implementation plan into bd beads following the gromit-decompose skill.

## Plan Content

%s

## Skill Instructions

%s

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, depends_on_index.

The spec label will be added automatically: spec:%s
`, planName, planBody, skillContent, planName)
}

// parsePriority converts priority string (P0, P1, P2) to int (0, 1, 2).
func parsePriority(p string) int {
	switch strings.ToUpper(p) {
	case "P0":
		return 0
	case "P1":
		return 1
	case "P2":
		return 2
	default:
		return 1 // Default to P1
	}
}
