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
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/validate"
	"github.com/danabrams/gromit/skills"
)

const (
	decomposePromptType         = "decompose"
	decomposeModel              = "sonnet"
	specLabelFormat             = "spec:%s"
	complexityHighLabel         = "complexity:high"
	estimatedFilesLabelFormat   = "estimated-files:%d"
	highComplexityFileThreshold = 5
	providerOutputPreviewLimit  = 500
)

// beadDef represents a bead definition from the provider's JSON output.
type beadDef struct {
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	Priority           string   `json:"priority"`
	EstimatedFiles     int      `json:"estimated_files,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	ExpectedOutputs    []string `json:"expected_outputs,omitempty"`
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

	maxRetries := normalizeMaxValidationRetries(input.MaxValidationRetries)
	currentPrompt := promptText
	model := decomposeModelForTier(input.Tier)

	var beadDefs []beadDef
	stats := &ValidationStats{}
	firstViolationCount := 0
	firstHighComplexityCount := 0
	bestHighComplexityCount := -1
	for attempt := 0; ; attempt++ {
		stats.Attempts++
		// Run provider non-interactively
		claudeResult, err := p.deps.ClaudeClient.Run(currentPrompt, model)
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
		if err := jsonutil.ExtractJSON(output, &beadDefs); err != nil {
			// Include truncated output in error for diagnostics
			preview := output
			if len(preview) > providerOutputPreviewLimit {
				preview = preview[:providerOutputPreviewLimit] + "... (truncated)"
			}
			return nil, fmt.Errorf("parsing bead definitions: %w\n\nProvider output:\n%s", err, preview)
		}

		highComplexityCount, _ := countComplexityByEstimate(beadDefs)
		priorBestHighComplexityCount := bestHighComplexityCount
		if attempt == 0 {
			firstHighComplexityCount = highComplexityCount
		}
		if bestHighComplexityCount == -1 || highComplexityCount < bestHighComplexityCount {
			bestHighComplexityCount = highComplexityCount
		}
		fmt.Print(formatComplexitySummaryLine(attempt+1, beadDefs))

		if input.SkipValidation {
			break
		}

		candidates := toBeadCandidates(beadDefs)
		validation := validatePipelineDecomposeCandidates(beadDefs)
		violations := validation.Violations
		violations = append(violations, validate.CheckBatchContract(candidates)...)
		stats.ViolationCount += len(violations)
		if attempt == 0 {
			firstViolationCount = len(violations)
		}
		if len(violations) == 0 {
			if highComplexityCount == 0 {
				if attempt > 0 {
					stats.Improved = true
					stats.SucceededAfterRetry = true
					fmt.Printf("Complexity clean exit after attempt %d: no high-complexity warning emitted.\n", attempt+1)
				}
				break
			}

			if attempt > 0 && priorBestHighComplexityCount >= 0 && highComplexityCount >= priorBestHighComplexityCount {
				if bestHighComplexityCount < firstHighComplexityCount {
					stats.Improved = true
				}
				stats.ProceededWithHighComplexityWarning = true
				details := make([]string, 0, len(beadDefs))
				for _, def := range beadDefs {
					if def.EstimatedFiles > highComplexityFileThreshold {
						details = append(details, fmt.Sprintf("%s (estimated_files=%d)", def.Title, def.EstimatedFiles))
					}
				}
				fmt.Printf(
					"Warning: high-complexity trajectory stalled at attempt %d; proceeding with current output. remaining=%d details=%s\n",
					attempt+1,
					highComplexityCount,
					strings.Join(details, ", "),
				)
				break
			}

			if attempt >= maxRetries {
				stats.RetryCapReached = true
				if bestHighComplexityCount < firstHighComplexityCount {
					stats.Improved = true
				}
				if !stats.Improved {
					stats.NonImprovingAtRetryCap = true
				}
				stats.ProceededWithHighComplexityWarning = true
				details := make([]string, 0, len(beadDefs))
				for _, def := range beadDefs {
					if def.EstimatedFiles > highComplexityFileThreshold {
						details = append(details, fmt.Sprintf("%s (estimated_files=%d)", def.Title, def.EstimatedFiles))
					}
				}
				fmt.Printf(
					"Warning: high-complexity beads remain after %d retr%s; proceeding with current output. remaining=%d details=%s\n",
					maxRetries,
					pluralizeRetry(maxRetries),
					highComplexityCount,
					strings.Join(details, ", "),
				)
				break
			}

			fmt.Printf("Retrying decomposition with complexity feedback (%d/%d)...\n", attempt+1, maxRetries)
			currentPrompt = promptText
			if complexityFeedback := buildComplexityRepromptFeedback(beadDefs); complexityFeedback != "" {
				currentPrompt += "\n\n" + complexityFeedback
			}
			continue
		}

		logValidationViolations(violations)

		if attempt >= maxRetries {
			if hasBatchContractViolation(violations) {
				return nil, fmt.Errorf("decomposition contract violation at retry cap: %s", formatViolations(violations))
			}
			stats.RetryCapReached = true
			if len(violations) < firstViolationCount {
				stats.Improved = true
			}
			if !stats.Improved {
				stats.NonImprovingAtRetryCap = true
			}
			fmt.Printf("Warning: validation still failing after %d retr%s; proceeding with current output.\n", maxRetries, pluralizeRetry(maxRetries))
			if highComplexityCount > 0 {
				stats.ProceededWithHighComplexityWarning = true
				details := make([]string, 0, len(beadDefs))
				for _, def := range beadDefs {
					if def.EstimatedFiles > highComplexityFileThreshold {
						details = append(details, fmt.Sprintf("%s (estimated_files=%d)", def.Title, def.EstimatedFiles))
					}
				}
				fmt.Printf(
					"Warning: high-complexity beads remain after %d retr%s; proceeding with current output. remaining=%d details=%s\n",
					maxRetries,
					pluralizeRetry(maxRetries),
					highComplexityCount,
					strings.Join(details, ", "),
				)
			}
			break
		}

		fmt.Printf("Retrying decomposition with validation feedback (%d/%d)...\n", attempt+1, maxRetries)
		currentPrompt = validate.BuildReprompt(promptText, candidates, violations)
		if complexityFeedback := buildComplexityRepromptFeedback(beadDefs); complexityFeedback != "" {
			currentPrompt += "\n\n" + complexityFeedback
		}
	}

	if len(beadDefs) == 0 {
		return nil, fmt.Errorf("no beads extracted from plan")
	}

	// Apply batch contract fallbacks: truncate if over max, error if under min
	var contractErr error
	beadDefs, contractErr = applyBatchContractFallbacks(beadDefs)
	if contractErr != nil {
		return nil, contractErr
	}

	// Review mode: return proposed beads without creating them
	if input.Review {
		result := NewDecomposeResult()
		result.PromptDiagnostics = promptDiagnostics
		result.ValidationStats = stats
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
	result.ValidationStats = stats
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

		// Use ExpectedOutputs when non-empty, falling back to AcceptanceCriteria
		criteria := def.AcceptanceCriteria
		if len(def.ExpectedOutputs) > 0 {
			criteria = def.ExpectedOutputs
		}

		// Create bead via BeadClient
		beadResult, err := p.deps.BeadClient.CreateWithDepsAndDescription(
			def.Title,
			priority,
			labels,
			criteria,
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

func countComplexityByEstimate(defs []beadDef) (high int, low int) {
	for _, def := range defs {
		if def.EstimatedFiles > highComplexityFileThreshold {
			high++
			continue
		}
		low++
	}
	return high, low
}

func formatComplexitySummaryLine(attempt int, defs []beadDef) string {
	highComplexityCount, lowComplexityCount := countComplexityByEstimate(defs)
	return fmt.Sprintf(
		"Complexity summary (attempt %d): high=%d low=%d high_titles=[%s]\n",
		attempt,
		highComplexityCount,
		lowComplexityCount,
		strings.Join(highComplexityTitles(defs), ", "),
	)
}

func buildComplexityRepromptFeedback(defs []beadDef) string {
	highTitles := make([]string, 0, len(defs))
	for _, def := range defs {
		if def.EstimatedFiles > highComplexityFileThreshold {
			highTitles = append(highTitles, fmt.Sprintf("- %s (estimated_files=%d)", def.Title, def.EstimatedFiles))
		}
	}
	if len(highTitles) == 0 {
		return ""
	}

	return "## Complexity Feedback\n\n" +
		"Complexity feedback:\n" +
		"The following beads are still high complexity. Reduce scope or split them into smaller beads.\n" +
		strings.Join(highTitles, "\n")
}

func highComplexityTitles(defs []beadDef) []string {
	titles := make([]string, 0, len(defs))
	for _, def := range defs {
		if def.EstimatedFiles > highComplexityFileThreshold {
			titles = append(titles, def.Title)
		}
	}
	return titles
}

func decomposeModelForTier(inputTier string) string {
	normalized := strings.TrimSpace(inputTier)
	if normalized == "" {
		return decomposeModel
	}
	return provider.TierToLegacyModel(normalized)
}

const decomposePromptTemplate = `# Decompose Plan: %s

You are decomposing an implementation plan into bd beads following the gromit-decompose skill.

## Plan Content

%s

## Skill Instructions

%s

## Output

Output ONLY a JSON array of bead definitions. No markdown, no explanations, no wrapper.
Each bead must include: title, description, priority, acceptance_criteria, expected_outputs, depends_on_index.

expected_outputs: list each individual deliverable, function, or independently testable item as a separate entry. These drive TDD RED-GREEN cycles — one cycle per entry. Do not summarize or group; enumerate fine-grained items.

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
		labels = append(labels, complexityHighLabel)
	}
	if estimatedFiles > 0 {
		labels = append(labels, fmt.Sprintf(estimatedFilesLabelFormat, estimatedFiles))
	}
	return labels
}

func toBeadCandidates(defs []beadDef) []validate.BeadCandidate {
	candidates := make([]validate.BeadCandidate, len(defs))
	for i, def := range defs {
		candidates[i] = validate.BeadCandidate{
			Title:              def.Title,
			Description:        def.Description,
			AcceptanceCriteria: def.AcceptanceCriteria,
			ExpectedOutputs:    def.ExpectedOutputs,
		}
	}
	return candidates
}

func validatePipelineDecomposeCandidates(defs []beadDef) validate.DecomposeOutputValidation {
	return validate.ValidateDecomposeOutput(
		toBeadCandidates(defs),
		validate.DecomposeValidationModePipeline,
		"",
	)
}

// applyBatchContractFallbacks enforces batch-level structural constraints on decompose output.
// If the batch exceeds maxSubBeads, it is truncated to maxSubBeads (with a warning).
// If the batch is below minSubBeads, an error is returned (cannot create missing beads mechanically).
func applyBatchContractFallbacks(defs []beadDef) ([]beadDef, error) {
	violations := validatePipelineDecomposeCandidates(defs).BatchViolations
	for _, v := range violations {
		switch v.Rule {
		case "batch_size_max":
			fmt.Printf("Warning: %s; truncating to %d beads.\n", v.Message, validate.MaxSubBeads)
			defs = defs[:validate.MaxSubBeads]
		case "batch_size_min":
			return nil, fmt.Errorf("decomposition contract violation: %s", v.Message)
		}
	}
	return defs, nil
}

func normalizeMaxValidationRetries(maxRetries int) int {
	if maxRetries < 0 {
		return 0
	}
	return maxRetries
}

func logValidationViolations(violations []validate.Violation) {
	fmt.Printf("Validation found %d violation(s) in decomposed beads.\n", len(violations))
	for _, violation := range violations {
		fmt.Printf("  - bead %d [%s]: %s\n", violation.BeadIndex, violation.Rule, violation.Message)
	}
}

func pluralizeRetry(count int) string {
	if count == 1 {
		return "y"
	}
	return "ies"
}

func hasBatchContractViolation(violations []validate.Violation) bool {
	for _, v := range violations {
		if v.Rule == "batch_size_min" || v.Rule == "batch_size_max" {
			return true
		}
	}
	return false
}

func formatViolations(violations []validate.Violation) string {
	rules := make([]string, 0, len(violations))
	for _, v := range violations {
		rules = append(rules, fmt.Sprintf("[%s] %s", v.Rule, v.Message))
	}
	return strings.Join(rules, ", ")
}
