package promptrender

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
