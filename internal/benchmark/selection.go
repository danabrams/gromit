package benchmark

import "fmt"

func ResolveSelectedBeads(manifestBeads []string, cliBeads []string, beadCount int) ([]string, error) {
	_ = beadCount
	selected := manifestBeads
	if len(cliBeads) > 0 {
		selected = cliBeads
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("selected cohort cannot be empty")
	}

	seen := make(map[string]struct{}, len(selected))
	for _, bead := range selected {
		if _, ok := seen[bead]; ok {
			return nil, fmt.Errorf("duplicate bead in selection: %q", bead)
		}
		seen[bead] = struct{}{}
	}

	return append([]string(nil), selected...), nil
}
