package scope

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/frontmatter"
)

// ResolveSpec returns a label array for a spec name.
// It constructs a label in the format "spec:name".
func ResolveSpec(specName string) []string {
	return []string{fmt.Sprintf("spec:%s", specName)}
}

// ValidateSpec checks if a spec file exists at <specsDir>/<specName>.md.
// If the file doesn't exist, it returns an error listing available spec names.
// Returns nil if the spec file exists.
func ValidateSpec(specsDir, specName string) error {
	// Check if the spec file exists
	specPath := filepath.Join(specsDir, specName+".md")
	if _, err := os.Stat(specPath); err == nil {
		return nil // Spec exists
	}

	// Spec doesn't exist - read directory to list available specs
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		// Directory doesn't exist or can't be read
		return fmt.Errorf("spec %q not found: %w", specName, err)
	}

	// Collect available .md file names (without extension)
	var availableSpecs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		availableSpecs = append(availableSpecs, name)
	}

	// Build error message
	if len(availableSpecs) == 0 {
		return fmt.Errorf("spec %q not found: no specs available in %s", specName, specsDir)
	}

	return fmt.Errorf("spec %q not found. Available specs: %s", specName, strings.Join(availableSpecs, ", "))
}

// ValidateFlags returns an error when more than one of epic, spec, or since flags are set.
// It considers trimmed values to handle whitespace-only inputs.
// When called with two arguments, since is treated as empty for backward compatibility.
func ValidateFlags(epic, spec string, since ...string) error {
	epicTrimmed := strings.TrimSpace(epic)
	specTrimmed := strings.TrimSpace(spec)
	sinceTrimmed := ""
	if len(since) > 0 {
		sinceTrimmed = strings.TrimSpace(since[0])
	}

	// Count non-empty flags
	count := 0
	if epicTrimmed != "" {
		count++
	}
	if specTrimmed != "" {
		count++
	}
	if sinceTrimmed != "" {
		count++
	}

	if count > 1 {
		return fmt.Errorf("--epic, --spec, and --since flags are mutually exclusive")
	}

	return nil
}

// ResolveEpic scans spec files in specsDir and returns labels for specs
// where the epic field matches epicID. Uses the spec id field for label construction.
func ResolveEpic(epicID, specsDir string) ([]string, error) {
	// Check if specsDir exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("specs directory does not exist: %s", specsDir)
	}

	// Read all .md files in specsDir
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	var labels []string
	for _, entry := range entries {
		// Skip non-.md files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		// Read frontmatter
		specPath := filepath.Join(specsDir, entry.Name())
		fm, _, err := frontmatter.ReadFile(specPath)
		if err != nil {
			// Skip files with invalid frontmatter
			continue
		}

		// Check if epic field matches
		epicValue, hasEpic := fm["epic"]
		if !hasEpic {
			continue
		}

		// Epic must be a string
		epicStr, ok := epicValue.(string)
		if !ok {
			continue
		}

		// Check if epic matches epicID
		if epicStr != epicID {
			continue
		}

		// Get the spec id field
		idValue, hasID := fm["id"]
		if !hasID {
			continue
		}

		// ID must be a string
		idStr, ok := idValue.(string)
		if !ok {
			continue
		}

		// Add label
		labels = append(labels, fmt.Sprintf("spec:%s", idStr))
	}

	return labels, nil
}
