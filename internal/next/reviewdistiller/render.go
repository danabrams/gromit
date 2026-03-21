package reviewdistiller

import (
	"bytes"
	"fmt"
	"strings"
)

// RenderMarkdown takes a DistillationResult and produces human-readable markdown.
// The output includes a header with run metadata and per-proposal sections detailing
// each proposal's type, confidence, narrative fields, and evidence references.
func RenderMarkdown(result *DistillationResult) string {
	var buf bytes.Buffer

	// Write header
	buf.WriteString(fmt.Sprintf("# Review Distillation: %s\n\n", result.Outcome))
	buf.WriteString("## Metadata\n\n")
	buf.WriteString(fmt.Sprintf("- **Run ID:** `%s`\n", result.RunID))
	buf.WriteString(fmt.Sprintf("- **Spec ID:** `%s`\n", result.SpecID))
	buf.WriteString(fmt.Sprintf("- **Outcome:** %s\n", formatOutcome(result.Outcome)))
	buf.WriteString(fmt.Sprintf("- **Model Tier:** %s\n", result.ModelTier))
	buf.WriteString(fmt.Sprintf("- **Created At:** %s\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
	buf.WriteString("\n")

	// Write proposals section
	if len(result.Proposals) == 0 {
		buf.WriteString("## Proposals\n\nNo proposals extracted.\n")
		return buf.String()
	}

	buf.WriteString(fmt.Sprintf("## Proposals (%d)\n\n", len(result.Proposals)))

	for i, proposal := range result.Proposals {
		buf.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, proposal.Title))
		buf.WriteString(fmt.Sprintf("**Type:** `%s` | **Confidence:** %s\n\n", proposal.Type, strings.ToUpper(proposal.Confidence)))

		// Confidence rationale if available
		if proposal.ConfidenceRationale != "" {
			buf.WriteString(fmt.Sprintf("_Confidence Rationale:_ %s\n\n", proposal.ConfidenceRationale))
		}

		// Narrative fields
		if proposal.WhatHappened != "" {
			buf.WriteString(fmt.Sprintf("**What Happened:**\n%s\n\n", proposal.WhatHappened))
		}

		if proposal.WhatWasMissing != "" {
			buf.WriteString(fmt.Sprintf("**What Was Missing:**\n%s\n\n", proposal.WhatWasMissing))
		}

		if proposal.ProposedChange != "" {
			buf.WriteString(fmt.Sprintf("**Proposed Change:**\n%s\n\n", proposal.ProposedChange))
		}

		if proposal.Rationale != "" {
			buf.WriteString(fmt.Sprintf("**Rationale:**\n%s\n\n", proposal.Rationale))
		}

		// Evidence references
		if len(proposal.EvidenceReferences) > 0 {
			buf.WriteString("**Evidence References:**\n")
			for _, ref := range proposal.EvidenceReferences {
				buf.WriteString(fmt.Sprintf("- %s\n", ref))
			}
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

// formatOutcome returns a human-readable formatted version of the outcome string.
// Converts snake_case to title case.
func formatOutcome(outcome string) string {
	parts := strings.Split(outcome, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(string(part[0])) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, " ")
}
