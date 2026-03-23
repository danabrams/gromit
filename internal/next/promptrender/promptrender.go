package promptrender

import (
	"fmt"
	"strings"

	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/playbook"
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

// MergedPlaybook loads playbook entries from both global and local directories,
// merges them with local-wins semantics, and returns the merged slice.
// This is the entry point for merged playbook resolution during prompt assembly.
func MergedPlaybook(globalDir, localDir string) ([]playbook.Entry, error) {
	return playbook.MergedPlaybook(globalDir, localDir)
}
