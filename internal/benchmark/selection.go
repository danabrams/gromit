package benchmark

import "fmt"

func ResolveSelectedBeads(manifestBeads []string, cliBeads []string, beadCount int) ([]string, error) {
	if beadCount < 0 {
		return nil, fmt.Errorf("--bead-count must be zero or greater")
	}

	selected := manifestBeads
	if len(cliBeads) > 0 {
		selected = cliBeads
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("selected cohort cannot be empty")
	}
	if beadCount > 0 {
		if beadCount > len(selected) {
			return nil, fmt.Errorf("--bead-count %d exceeds selected cohort size %d", beadCount, len(selected))
		}
		selected = selected[:beadCount]
	}
	if len(selected) != 5 {
		return nil, fmt.Errorf("selected cohort size %d must be exactly 5", len(selected))
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
