package jsonutil

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ExtractObject extracts a JSON object from text and unmarshals it into the target.
// It tries multiple strategies:
// 1. Direct unmarshaling (pure JSON)
// 2. Find first { and last } and extract that substring
func ExtractObject(text string, target any) error {
	if text == "" {
		return fmt.Errorf("extract object: input is empty")
	}

	// Try direct unmarshaling first
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), target); err == nil {
		return nil
	}

	// Find JSON object by matching { and }
	start := strings.Index(text, "{")
	if start == -1 {
		return fmt.Errorf("extract object: no JSON object found (no opening brace)")
	}

	// Find the matching closing bracket
	jsonStr := extractBracketedJSON(text[start:], '{', '}')
	if jsonStr == "" {
		return fmt.Errorf("extract object: malformed JSON object (missing closing brace)")
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("extract object: %w", err)
	}

	return nil
}

// ExtractArray extracts a JSON array from text and unmarshals it into the target.
// It tries multiple strategies:
// 1. Direct unmarshaling (pure JSON)
// 2. Find first [ and matching ] and extract that substring
func ExtractArray(text string, target any) error {
	if text == "" {
		return fmt.Errorf("extract array: input is empty")
	}

	// Try direct unmarshaling first
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), target); err == nil {
		return nil
	}

	// Find JSON array by matching [ and ]
	start := strings.Index(text, "[")
	if start == -1 {
		return fmt.Errorf("extract array: no JSON array found (no opening bracket)")
	}

	// Find the matching closing bracket
	jsonStr := extractBracketedJSON(text[start:], '[', ']')
	if jsonStr == "" {
		return fmt.Errorf("extract array: malformed JSON array (missing closing bracket)")
	}

	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("extract array: %w", err)
	}

	return nil
}

// ExtractCodeBlock extracts JSON from a ```json code block.
// Matches patterns like:
//
//	```json
//	{...}
//	```
//
// or
//
//	```
//	{...}
//	```
func ExtractCodeBlock(text string, target any) error {
	if text == "" {
		return fmt.Errorf("extract code block: input is empty")
	}

	// Match ```json...``` or ```...``` blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n(.*?)\\n```")
	matches := re.FindStringSubmatch(text)
	if len(matches) < 2 {
		return fmt.Errorf("extract code block: no code block found")
	}

	jsonStr := strings.TrimSpace(matches[1])
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("extract code block: %w", err)
	}

	return nil
}

// ExtractJSON is a convenience function that tries multiple strategies in order:
// 1. Code block extraction (for ```json...``` format)
// 2. Direct parsing (for pure JSON)
// 3. Array extraction (for [...] format)
// 4. Object extraction (for {...} format)
// Returns the first successful extraction, or an error if all strategies fail.
func ExtractJSON(text string, target any) error {
	if text == "" {
		return fmt.Errorf("extract json: input is empty")
	}

	// Try code block first (most specific)
	if err := ExtractCodeBlock(text, target); err == nil {
		return nil
	}

	// Try direct parsing (pure JSON)
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), target); err == nil {
		return nil
	}

	// Try array extraction (look for [ first since it's more specific than {)
	if err := ExtractArray(text, target); err == nil {
		return nil
	}

	// Try object extraction (more general)
	if err := ExtractObject(text, target); err == nil {
		return nil
	}

	return fmt.Errorf("extract json: could not extract JSON using any strategy")
}

// extractBracketedJSON finds matching opening and closing brackets in text.
// Handles nested structures and string escaping.
// openBracket and closeBracket should be matching pairs like '{' and '}' or '[' and ']'.
// Assumes text starts with openBracket.
// Returns the bracketed content including brackets, or empty string if malformed.
func extractBracketedJSON(text string, openBracket, closeBracket byte) string {
	if len(text) == 0 || text[0] != openBracket {
		return ""
	}

	bracketCount := 0
	inString := false
	escapeNext := false

	for i := 0; i < len(text); i++ {
		char := text[i]

		if escapeNext {
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' && (i == 0 || text[i-1] != '\\') {
			inString = !inString
			continue
		}

		if !inString {
			if char == openBracket {
				bracketCount++
			} else if char == closeBracket {
				bracketCount--
				if bracketCount == 0 {
					return text[:i+1]
				}
			}
		}
	}

	// Malformed: didn't find matching close bracket
	return ""
}
