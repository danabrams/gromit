package methodology

import (
	"fmt"
	"strings"
)

// ResolveAdapter returns the appropriate RunnerAdapter for the given profile.
// It maps "go" to GoAdapter and "node", "python", "custom" to PassthroughAdapter.
func ResolveAdapter(profile string) (RunnerAdapter, error) {
	normalized := strings.ToLower(strings.TrimSpace(profile))

	switch normalized {
	case "go":
		return GoAdapter{}, nil
	case "node", "python", "custom":
		return PassthroughAdapter{}, nil
	default:
		return nil, fmt.Errorf("unsupported profile %q", profile)
	}
}
