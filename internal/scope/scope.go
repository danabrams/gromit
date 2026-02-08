package scope

import (
	"fmt"
	"strings"
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
