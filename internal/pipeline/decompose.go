package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/skills"
)

const (
	decomposePromptType         = "decompose"
	specLabelFormat             = "spec:%s"
	highComplexityFileThreshold = 5
)

// beadDef represents a bead definition from the provider's JSON output.
type beadDef struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Priority           string   `json:"priority"`
	EstimatedFiles     int      `json:"estimated_files,omitempty"`
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
	promptText, promptDiagnostics := buildDecomposePrompt(input.PlanName, planBody, skills.DecomposeSkill)

	// Run provider non-interactively
	claudeResult, err := p.deps.ClaudeClient.Run(promptText, "sonnet")
	if err != nil {
		return nil, fmt.Errorf("invoking provider: %w", err)
	}

	// Extract output from typed result
	if !claudeResult.Success {
		return nil, fmt.Errorf("provider invocation failed (exit code %d)\nOutput:\n%s", claudeResult.ExitCode, claudeResult.Output)
	}
	output := strings.TrimSpace(claudeResult.Output)
	if output == "" {
		return nil, fmt.Errorf("provider returned empty output for decompose; check CLI connectivity and retry")
	}
	var beadDefs []beadDef
	if err := jsonutil.ExtractJSON(output, &beadDefs); err != nil {
		// Include truncated output in error for diagnostics
		preview := output
		if len(preview) > 500 {
			preview = preview[:500] + "... (truncated)"
		}
		return nil, fmt.Errorf("parsing bead definitions: %w\n\nProvider output:\n%s", err, preview)
	}

	if len(beadDefs) == 0 {
		return nil, fmt.Errorf("no beads extracted from plan")
	}

	// Review mode: return proposed beads without creating them
	if input.Review {
		result := NewDecomposeResult()
		result.PromptDiagnostics = promptDiagnostics
		for _, def := range beadDefs {
			bead := newCreatedBeadFromDef(def, input.PlanName, "")
			result.CreatedBeads = append(result.CreatedBeads, bead)
		}
		result.PlanUpdated = false
		return &result, nil
	}

	// Create beads
	result := NewDecomposeResult()
	result.PromptDiagnostics = promptDiagnostics
	createdIDs := []string{}

	for i, def := range beadDefs {
		// Map priority string to int
		priority := parsePriority(def.Priority)

		labels := buildDecomposeLabels(input.PlanName, def.EstimatedFiles)

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

		// Extract ID from typed bead creation result
		createdIDs = append(createdIDs, beadResult.ID)

		// Add to result
		bead := newCreatedBeadFromDef(def, input.PlanName, beadResult.ID)
		result.CreatedBeads = append(result.CreatedBeads, bead)
	}

	// Update plan frontmatter
	updates := map[string]interface{}{
		"decomposed":    true,
		"decomposed_at": time.Now().Format(time.RFC3339),
	}
	if err := frontmatter.UpdateFile(planPath, updates); err != nil {
		return nil, fmt.Errorf("updating plan frontmatter: %w", err)
	}

	result.PlanUpdated = true
	return &result, nil
}

const decomposePromptTemplate = `# Decompose Plan: %s

You are decomposing an implementation plan into bd beads following the gromit-decompose skill.

## Plan Content

%s

## Skill Instructions

%s

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, depends_on_index.

The spec label will be added automatically: spec:%s
`

func decomposeTemplateStatic(planName string) string {
	return fmt.Sprintf(decomposePromptTemplate, planName, "", "", planName)
}

// buildDecomposePrompt constructs the prompt for the configured provider.
func buildDecomposePrompt(planName, planBody, skillContent string) (string, *prompt.PromptDiagnostics) {
	promptText := fmt.Sprintf(decomposePromptTemplate, planName, planBody, skillContent, planName)
	templateStatic := decomposeTemplateStatic(planName)

	diagnostics := prompt.NewDiagnostics(decomposePromptType, map[string]int{
		prompt.SectionPlanBody:          prompt.EstimateTokens(planBody),
		prompt.SectionSkillInstructions: prompt.EstimateTokens(skillContent),
		prompt.SectionTemplateStatic:    prompt.EstimateTokens(templateStatic),
	})

	return promptText, diagnostics
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

// newCreatedBeadFromDef creates a CreatedBead from a beadDef.
// If beadID is empty, the bead hasn't been created yet (review mode).
func newCreatedBeadFromDef(def beadDef, planName, beadID string) CreatedBead {
	bead := NewCreatedBead()
	bead.ID = beadID
	bead.Title = def.Title
	bead.Priority = parsePriority(def.Priority)
	bead.Labels = buildDecomposeLabels(planName, def.EstimatedFiles)
	return bead
}

func specLabel(planName string) string {
	return fmt.Sprintf(specLabelFormat, planName)
}

func buildDecomposeLabels(planName string, estimatedFiles int) []string {
	labels := []string{specLabel(planName)}
	if estimatedFiles > highComplexityFileThreshold {
		labels = append(labels, "complexity:high")
	}
	if estimatedFiles > 0 {
		labels = append(labels, fmt.Sprintf("estimated-files:%d", estimatedFiles))
	}
	return labels
}
