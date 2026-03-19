package reviewpacket

import (
	"fmt"
	"strings"
)

// RenderProductReview converts a ProductReview to human-readable markdown.
func RenderProductReview(pr ProductReview) string {
	var buf strings.Builder

	// Title
	if pr.SpecTitle != "" {
		fmt.Fprintf(&buf, "# %s\n\n", pr.SpecTitle)
	}

	// Terminal state
	if pr.TerminalState != "" {
		fmt.Fprintf(&buf, "**Status:** %s\n\n", pr.TerminalState)
	}

	// Diagnostic flag
	if pr.IsDiagnostic {
		buf.WriteString("⚠️ This is a diagnostic review\n\n")
	}

	// Summary
	if pr.Summary != "" {
		fmt.Fprintf(&buf, "## Summary\n%s\n\n", pr.Summary)
	}

	// Behavior cards section
	if len(pr.BehaviorCards) > 0 {
		buf.WriteString("## Behavior Cards\n\n")
		for _, card := range pr.BehaviorCards {
			fmt.Fprintf(&buf, "### %s (%s)\n", card.Title, card.ID)
			fmt.Fprintf(&buf, "**Status:** %s\n\n", card.AutomaticStatus)

			if card.Given != "" || card.When != "" || card.Then != "" {
				buf.WriteString("**Specification:**\n")
				if card.Given != "" {
					fmt.Fprintf(&buf, "- **Given:** %s\n", card.Given)
				}
				if card.When != "" {
					fmt.Fprintf(&buf, "- **When:** %s\n", card.When)
				}
				if card.Then != "" {
					fmt.Fprintf(&buf, "- **Then:** %s\n", card.Then)
				}
				buf.WriteString("\n")
			}

			if len(card.EvidenceFiles) > 0 {
				buf.WriteString("**Evidence Files:**\n")
				for _, file := range card.EvidenceFiles {
					fmt.Fprintf(&buf, "- %s\n", file)
				}
				buf.WriteString("\n")
			}

			if card.Notes != "" {
				fmt.Fprintf(&buf, "**Notes:** %s\n\n", card.Notes)
			}
		}
	}

	// Surprises section
	if len(pr.Surprises) > 0 {
		buf.WriteString("## Surprises\n\n")
		for _, surprise := range pr.Surprises {
			fmt.Fprintf(&buf, "- %s\n", surprise)
		}
		buf.WriteString("\n")
	}

	// Blocker section
	if pr.BlockerSummary != "" {
		fmt.Fprintf(&buf, "## Blocker\n%s\n\n", pr.BlockerSummary)
	}

	// Recommended action
	if pr.RecommendedNextAction != "" {
		fmt.Fprintf(&buf, "## Recommended Next Action\n%s\n", pr.RecommendedNextAction)
	}

	return strings.TrimSpace(buf.String())
}

// RenderProcessReview converts a ProcessReview to human-readable markdown.
func RenderProcessReview(pr ProcessReview) string {
	var buf strings.Builder

	// Trust level
	fmt.Fprintf(&buf, "# Process Review\n\n")
	fmt.Fprintf(&buf, "**Trust Level:** %s\n\n", pr.TrustLevel)

	// Evidence sections
	if pr.AutomaticProof != "" {
		fmt.Fprintf(&buf, "## Automatic Proof\n%s\n\n", pr.AutomaticProof)
	}

	if pr.MachineReview != "" {
		fmt.Fprintf(&buf, "## Machine Review\n%s\n\n", pr.MachineReview)
	}

	if pr.Acceptance != "" {
		fmt.Fprintf(&buf, "## Acceptance\n%s\n\n", pr.Acceptance)
	}

	// Degraded flags
	if len(pr.DegradedFlags) > 0 {
		buf.WriteString("## Degraded Flags\n\n")
		for _, flag := range pr.DegradedFlags {
			fmt.Fprintf(&buf, "- %s\n", flag)
		}
		buf.WriteString("\n")
	}

	// Repair cycles
	if pr.RepairCycles > 0 {
		fmt.Fprintf(&buf, "**Repair Cycles:** %d\n\n", pr.RepairCycles)
	}

	// Repeated failure flag
	if pr.RepeatedFailureFlag {
		buf.WriteString("⚠️ repeated failures detected\n\n")
	}

	// Recommended posture
	if pr.RecommendedPosture != "" {
		fmt.Fprintf(&buf, "## Recommended Posture\n%s\n", pr.RecommendedPosture)
	}

	return strings.TrimSpace(buf.String())
}
