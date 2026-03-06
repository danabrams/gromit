package generation

import (
	"strconv"
	"strings"
)

const labelPrefix = "gen:"

// Current returns the highest generation label parsed from the provided labels.
// When no generation label exists, it returns 0.
func Current(labels []string) int {
	maxGen := 0
	for _, label := range labels {
		if !strings.HasPrefix(label, labelPrefix) {
			continue
		}
		value := strings.TrimPrefix(label, labelPrefix)
		if gen, err := strconv.Atoi(value); err == nil && gen > maxGen {
			maxGen = gen
		}
	}
	return maxGen
}

// Format returns a generation label for the given generation number.
func Format(generation int) string {
	return labelPrefix + strconv.Itoa(generation)
}
