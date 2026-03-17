package planner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParsePlan extracts a Plan from raw agent output. It handles both bare JSON
// and JSON wrapped in markdown code fences (```json ... ```).
func ParsePlan(raw string) (Plan, error) {
	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return Plan{}, fmt.Errorf("no JSON found in agent output")
	}

	var p Plan
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		return Plan{}, fmt.Errorf("invalid plan JSON: %w", err)
	}

	p.NormalizeNilFields()

	if err := ValidatePlan(p); err != nil {
		return Plan{}, fmt.Errorf("plan validation failed: %w", err)
	}

	return p, nil
}

// extractJSON tries to find JSON in the raw string, first looking for
// markdown code fences, then trying the whole string as JSON.
func extractJSON(raw string) string {
	// Try markdown fence extraction first.
	if idx := strings.Index(raw, "```json"); idx >= 0 {
		start := idx + len("```json")
		end := strings.Index(raw[start:], "```")
		if end >= 0 {
			return strings.TrimSpace(raw[start : start+end])
		}
	}
	if idx := strings.Index(raw, "```"); idx >= 0 {
		start := idx + len("```")
		end := strings.Index(raw[start:], "```")
		if end >= 0 {
			candidate := strings.TrimSpace(raw[start : start+end])
			if len(candidate) > 0 && candidate[0] == '{' {
				return candidate
			}
		}
	}

	// Try bare JSON.
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		return trimmed
	}

	// Fallback: scan for first '{' and extract a balanced JSON object.
	if obj := extractBalancedObject(raw); obj != "" {
		return obj
	}

	return ""
}

// extractBalancedObject finds the first '{' in s and extracts a balanced JSON
// object by counting braces, correctly skipping braces inside quoted strings.
func extractBalancedObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(s); i++ {
		c := s[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return ""
}
