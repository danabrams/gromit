package reviewdistiller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// BuildPrompt constructs a complete prompt for the distiller by combining
// a shared preamble with outcome-specific instructions.
func BuildPrompt(inputs *DistillerInputs, outcome string) string {
	preamble := buildPreamble(inputs)
	outcomeInstructions := buildOutcomeInstructions(outcome)

	return preamble + "\n\n" + outcomeInstructions
}

// buildPreamble constructs the shared preamble that includes the spec content
// and all artifact JSON artifacts formatted for clarity.
func buildPreamble(inputs *DistillerInputs) string {
	preambleTmpl := `# Spec Content
{{.SpecContent}}

# Artifacts

## Review Outcome
{{.ReviewOutcomeJSON}}

## Product Review
{{.ProductReviewJSON}}

## Process Review
{{.ProcessReviewJSON}}

## Manual Checklist
{{.ManualChecklistJSON}}

## Validation
{{.ValidationJSON}}

## Acceptance
{{.AcceptanceJSON}}

## Machine Review
{{.MachineReviewJSON}}

## Task Results
{{.TaskResultsJSON}}

## Run Metadata
{{.RunMetadataJSON}}`

	data := map[string]string{
		"SpecContent":         inputs.SpecContent,
		"ReviewOutcomeJSON":   formatJSON(inputs.ReviewOutcome),
		"ProductReviewJSON":   formatJSON(inputs.ProductReview),
		"ProcessReviewJSON":   formatJSON(inputs.ProcessReview),
		"ManualChecklistJSON": formatJSON(inputs.ManualChecklist),
		"ValidationJSON":      formatJSON(inputs.Validation),
		"AcceptanceJSON":      formatJSON(inputs.Acceptance),
		"MachineReviewJSON":   formatJSON(inputs.MachineReview),
		"TaskResultsJSON":     formatJSON(inputs.TaskResults),
		"RunMetadataJSON":     formatJSON(inputs.RunMetadata),
	}

	tmpl := template.Must(template.New("preamble").Parse(preambleTmpl))
	var buf bytes.Buffer
	tmpl.Execute(&buf, data)

	return buf.String()
}

// buildOutcomeInstructions returns outcome-specific instruction text.
func buildOutcomeInstructions(outcome string) string {
	switch outcome {
	case "accepted":
		return buildAcceptedInstructions()
	case "rework_implementation_gap":
		return buildReworkImplementationGapInstructions()
	case "rework_vision_change":
		return buildReworkVisionChangeInstructions()
	default:
		return fmt.Sprintf("# Unknown Outcome: %s", outcome)
	}
}

// buildAcceptedInstructions returns instructions for the accepted outcome.
// The review was accepted; extract doctrine_rule and planner_heuristic proposals
// that would improve future implementations.
func buildAcceptedInstructions() string {
	return `# Instructions for Accepted Outcome

The review outcome is accepted. Extract proposals that represent improvements
to doctrine, heuristics, or process that should inform future implementations.

## Proposal Types Required
- doctrine_rule: Fundamental principles or rules that should guide future work
- planner_heuristic: Planning shortcuts or heuristics that improve efficiency

Extract at least one proposal of these types from the review feedback.`
}

// buildReworkImplementationGapInstructions returns instructions for the rework_implementation_gap outcome.
// Work needs rework due to implementation gaps; extract validation_gap, doctrine_rule, or planner_heuristic.
func buildReworkImplementationGapInstructions() string {
	return `# Instructions for rework_implementation_gap Outcome

The review outcome is rework_implementation_gap: rework is needed due to an implementation gap.
The spec or vision was clear, but execution fell short.

## Proposal Types Required
- validation_gap: Gaps in validation or testing that must be addressed
- doctrine_rule: Fundamental principles or rules that were violated
- planner_heuristic: Planning or process issues that contributed to the gap

Extract at least one proposal of these types from the review feedback.`
}

// buildReworkVisionChangeInstructions returns instructions for the rework_vision_change outcome.
// Work needs rework due to vision or scope changes; extract refinement_guidance.
func buildReworkVisionChangeInstructions() string {
	return `# Instructions for rework_vision_change Outcome

The review outcome is rework_vision_change: rework is needed due to a change in vision, scope, or requirements.
The original approach was sound, but the target has shifted.

## Proposal Types Required
- refinement_guidance: Guidance on how to refine the vision or adjust scope

Extract at least one proposal of this type from the review feedback.`
}

// formatJSON formats a JSON RawMessage for inclusion in prompts.
// Returns "(no data)" if the input is nil or empty.
func formatJSON(data json.RawMessage) string {
	if len(data) == 0 {
		return "(no data)"
	}
	return string(data)
}
