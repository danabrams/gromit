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
// 2. Find { and matching } and extract that substring, trying subsequent positions on failure
func ExtractObject(text string, target any) error {
	if text == "" {
		return fmt.Errorf("extract object: input is empty")
	}

	// Try direct unmarshaling first
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), target); err == nil {
		return nil
	}

	// Try each { position until one produces valid JSON
	var lastErr error
	searchFrom := 0
	for {
		idx := strings.Index(text[searchFrom:], "{")
		if idx == -1 {
			break
		}
		start := searchFrom + idx

		jsonStr := extractBracketedJSON(text[start:], '{', '}')
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), target); err == nil {
				return nil
			} else {
				lastErr = fmt.Errorf("extract object: %w", err)
			}
		}

		searchFrom = start + 1
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("extract object: no JSON object found (no opening brace)")
}

// ExtractArray extracts a JSON array from text and unmarshals it into the target.
// It tries multiple strategies:
// 1. Direct unmarshaling (pure JSON)
// 2. Find [ and matching ] and extract that substring, trying subsequent positions on failure
func ExtractArray(text string, target any) error {
	if text == "" {
		return fmt.Errorf("extract array: input is empty")
	}

	// Try direct unmarshaling first
	text = strings.TrimSpace(text)
	if err := json.Unmarshal([]byte(text), target); err == nil {
		return nil
	}

	// Try each [ position until one produces valid JSON
	var lastErr error
	searchFrom := 0
	for {
		idx := strings.Index(text[searchFrom:], "[")
		if idx == -1 {
			break
		}
		start := searchFrom + idx

		jsonStr := extractBracketedJSON(text[start:], '[', ']')
		if jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), target); err == nil {
				return nil
			} else {
				lastErr = fmt.Errorf("extract array: %w", err)
			}
		}

		searchFrom = start + 1
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("extract array: no JSON array found (no opening bracket)")
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

		if inString && char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
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
