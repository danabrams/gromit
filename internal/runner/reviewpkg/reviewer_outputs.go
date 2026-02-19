package reviewpkg

import "strings"

func expectedOutputsOrTitle(outputs []string, title string) []string {
	if len(outputs) > 0 {
		return outputs
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return []string{}
	}
	return []string{trimmedTitle}
}
