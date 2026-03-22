package playbook

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/doctrine"
)

// FormatDoctrineForPrompt renders a slice of doctrine rules as a markdown list
// suitable for prompt injection. Format: `- **<summary>** (scope: <scope>)`
func FormatDoctrineForPrompt(rules []doctrine.Rule) string {
	if len(rules) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, rule := range rules {
		sb.WriteString(fmt.Sprintf("- **%s** (scope: %s)\n", rule.Summary, rule.Scope))
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// FormatPlaybookForPrompt renders a slice of playbook entries as a markdown list
// suitable for prompt injection. Format: `- **<title>**: <content>` with rationale on the next line.
func FormatPlaybookForPrompt(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, entry := range entries {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", entry.Title, entry.Content))
		if entry.Rationale != "" {
			sb.WriteString(fmt.Sprintf("  Rationale: %s\n", entry.Rationale))
		}
		// Add blank line between entries except after the last one
		if i < len(entries)-1 {
			sb.WriteString("\n")
		}
	}

	return strings.TrimSuffix(sb.String(), "\n")
}
