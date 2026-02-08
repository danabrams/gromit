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

// ValidateFlags returns an error when both epic and spec flags are set.
// It considers trimmed values to handle whitespace-only inputs.
func ValidateFlags(epic, spec string) error {
	epicTrimmed := strings.TrimSpace(epic)
	specTrimmed := strings.TrimSpace(spec)

	if epicTrimmed != "" && specTrimmed != "" {
		return fmt.Errorf("--epic and --spec flags are mutually exclusive")
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
