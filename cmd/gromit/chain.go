package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
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

// execGromit executes the current gromit binary as a subprocess with the given arguments.
// It connects the subprocess's stdin/stdout/stderr to the parent process's stdio.
// Returns nil if the subprocess exits successfully or exits with an error code
// (subprocess printed its own errors). Returns non-nil only if the subprocess
// cannot be launched.
func execGromit(args ...string) error {
	// Find the current binary path
	binary, err := os.Executable()
	if err != nil {
		// Fallback to os.Args[0] if os.Executable() fails
		binary = os.Args[0]
	}

	// Create command
	cmd := exec.Command(binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the subprocess
	if err := cmd.Run(); err != nil {
		// Treat exec.ExitError as nil - subprocess printed its own errors
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
		return err
	}

	return nil
}
