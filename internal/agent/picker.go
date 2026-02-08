package agent

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Pick prompts the user to select an agent from the provided list.
// Returns the selected agent name or an error.
//
// If only one agent is available, returns it immediately without prompting.
// If input is empty and a default is available (and in the list), returns the default.
// Invalid input causes re-prompting until a valid choice is made.
//
// Parameters:
//   - agents: List of available agent names
//   - defaultAgent: Default agent name (may be empty or not in list)
//   - r: Reader for user input
//   - w: Writer for prompt output
func Pick(agents []string, defaultAgent string, r io.Reader, w io.Writer) (string, error) {
	// Validate input
	if agents == nil || len(agents) == 0 {
		return "", fmt.Errorf("agents list cannot be empty")
	}

	// Single agent: return immediately without prompting
	if len(agents) == 1 {
		return agents[0], nil
	}

	// Check if default is in the list
	defaultIndex := -1
	for i, agent := range agents {
		if agent == defaultAgent {
			defaultIndex = i
			break
		}
	}

	scanner := bufio.NewScanner(r)

	for {
		// Display numbered list
		for i, agent := range agents {
			if i == defaultIndex {
				fmt.Fprintf(w, "  %d. %s (default)\n", i+1, agent)
			} else {
				fmt.Fprintf(w, "  %d. %s\n", i+1, agent)
			}
		}

		// Display choice prompt
		fmt.Fprintf(w, "\nChoice [1-%d]: ", len(agents))

		// Read input
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return "", fmt.Errorf("reading input: %w", err)
			}
			// EOF without valid input
			return "", fmt.Errorf("unexpected EOF")
		}

		input := strings.TrimSpace(scanner.Text())

		// Empty input: use default if available
		if input == "" {
			if defaultIndex >= 0 {
				return agents[defaultIndex], nil
			}
			// No valid default, re-prompt
			continue
		}

		// Parse numeric choice
		choice, err := strconv.Atoi(input)
		if err != nil {
			// Non-numeric input, re-prompt
			continue
		}

		// Validate choice is in range
		if choice < 1 || choice > len(agents) {
			// Out of range, re-prompt
			continue
		}

		// Valid choice
		return agents[choice-1], nil
	}
}
