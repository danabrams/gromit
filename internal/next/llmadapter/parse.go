package llmadapter

import "strings"

// ExtractJSON extracts the first JSON object ({...}) or array ([...]) from
// raw LLM output. It first strips markdown code fences if present, then scans
// for the first '{' or '[' and finds the matching closing bracket using a
// bracket-counting approach that handles nested structures.
func ExtractJSON(output string) string {
	// Step 1: Strip markdown fences if present.
	stripped := output
	if idx := strings.Index(output, "```json"); idx >= 0 {
		start := idx + len("```json")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			stripped = strings.TrimSpace(output[start : start+end])
		}
	} else if idx := strings.Index(output, "```"); idx >= 0 {
		start := idx + len("```")
		if end := strings.Index(output[start:], "```"); end >= 0 {
			stripped = strings.TrimSpace(output[start : start+end])
		}
	}

	// Step 2: Scan for the first '{' or '[' and find its matching closer.
	return extractBracketedJSON(stripped)
}

// extractBracketedJSON finds the first '{' or '[' in s, then uses bracket
// counting to locate the matching '}' or ']', returning the substring.
// Returns s unchanged (trimmed) if no bracket pair is found.
func extractBracketedJSON(s string) string {
	startIdx := -1
	var open, close byte
	for i := 0; i < len(s); i++ {
		if s[i] == '{' || s[i] == '[' {
			startIdx = i
			open = s[i]
			if open == '{' {
				close = '}'
			} else {
				close = ']'
			}
			break
		}
	}
	if startIdx < 0 {
		return strings.TrimSpace(s)
	}

	depth := 0
	inString := false
	escaped := false
	for i := startIdx; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == open {
			depth++
		} else if ch == close {
			depth--
			if depth == 0 {
				return s[startIdx : i+1]
			}
		}
	}
	// No matching close found — return trimmed input as fallback.
	return strings.TrimSpace(s)
}
