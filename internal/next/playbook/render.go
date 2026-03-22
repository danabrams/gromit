package playbook

import (
	"fmt"
	"strings"
)

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

// RenderPlaybookSection renders a slice of active playbook entries as a markdown-formatted
// string with title, content, and rationale per entry. Suitable for prompt injection.
// Only includes entries with status="active" and excludes superseded entries.
func RenderPlaybookSection(entries []Entry) string {
	active := ActiveEntries(entries)
	return FormatPlaybookForPrompt(active)
}
