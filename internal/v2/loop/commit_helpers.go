package loop

import "strings"

func capitalizedDecision(decision string) string {
	trimmed := strings.TrimSpace(decision)
	if trimmed == "" {
		return trimmed
	}
	lower := strings.ToLower(trimmed)
	if len(lower) == 1 {
		return strings.ToUpper(lower)
	}
	return strings.ToUpper(lower[:1]) + lower[1:]
}
