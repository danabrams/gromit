package main

import (
	"bufio"
	"fmt"
	"strings"
)

// confirmPrompt prints a yes/no prompt and reads the user's response.
// Returns true for yes, false for no.
// If the user provides empty input, returns defaultYes.
// Accepts y/yes/n/no case-insensitively.
func confirmPrompt(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	// Build prompt suffix based on default
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}

	// Print prompt
	fmt.Print(prompt + suffix)

	// Read line
	line, err := reader.ReadString('\n')
	if err != nil {
		// On error, return default
		return defaultYes
	}

	// Trim whitespace and convert to lowercase
	response := strings.ToLower(strings.TrimSpace(line))

	// Empty input uses default
	if response == "" {
		return defaultYes
	}

	// Check for yes/no
	if response == "y" || response == "yes" {
		return true
	}
	if response == "n" || response == "no" {
		return false
	}

	// Invalid input: return default
	return defaultYes
}
