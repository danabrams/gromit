package benchmark

import "fmt"

type BeadLookup interface{}

func ValidateSelectedCohort(_ BeadLookup, selected []string, minSize int) ([]string, error) {
	if len(selected) < minSize {
		return nil, fmt.Errorf("selected cohort size %d is below minimum %d", len(selected), minSize)
	}
	return append([]string(nil), selected...), nil
}
