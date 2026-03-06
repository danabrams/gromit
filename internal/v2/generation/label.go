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
		if gen, ok := parseGenerationLabel(label); ok && gen > maxGen {
			maxGen = gen
		}
	}
	return maxGen
}

// Format returns a generation label for the given generation number.
func Format(generation int) string {
	return labelPrefix + strconv.Itoa(generation)
}

func parseGenerationLabel(label string) (int, bool) {
	if !strings.HasPrefix(label, labelPrefix) {
		return 0, false
	}
	value := strings.TrimPrefix(label, labelPrefix)
	gen, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return gen, true
}
