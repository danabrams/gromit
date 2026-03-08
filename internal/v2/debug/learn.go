package debug

import "strings"

// LearningExtractionInput is the raw learn/recommendation output from debug LLM.
type LearningExtractionInput struct {
	LearningsEntry         string
	SystemicRecommendation string
	RootCause              RootCause
}

// LearningExtraction is the normalized learning output for debug handling.
type LearningExtraction struct {
	LearningsEntry         string
	SystemicRecommendation string
	Autonomous             bool
}

const autonomousLearningMarker = "*Autonomous: true*"

// ExtractLearning classifies output into autonomous learnings vs systemic recommendations.
func ExtractLearning(input LearningExtractionInput) LearningExtraction {
	learningsEntry := strings.TrimSpace(input.LearningsEntry)
	systemicRecommendation := strings.TrimSpace(input.SystemicRecommendation)

	if systemicRecommendation == "" && isSystemicLearning(learningsEntry) {
		systemicRecommendation = BuildSystemicRecommendation(input.RootCause, learningsEntry)
		if systemicRecommendation == "" {
			systemicRecommendation = learningsEntry
		}
		learningsEntry = ""
	}
	if systemicRecommendation == "" {
		systemicRecommendation = BuildSystemicRecommendation(input.RootCause, learningsEntry)
	}

	if systemicRecommendation != "" {
		return LearningExtraction{
			SystemicRecommendation: systemicRecommendation,
		}
	}

	if learningsEntry == "" {
		learningsEntry = buildLearningEntryFromRootCause(input.RootCause)
	}
	if learningsEntry == "" {
		return LearningExtraction{}
	}
	learningsEntry = withAutonomousMarker(learningsEntry)

	return LearningExtraction{
		LearningsEntry: learningsEntry,
		Autonomous:     true,
	}
}

func isSystemicLearning(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}

	keywords := []string{
		"prompt fragment",
		"code guard",
		"process change",
		"rule update",
		"rules.md",
		"pipeline guard",
	}

	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}

func withAutonomousMarker(entry string) string {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return trimmed
	}
	marker := strings.ToLower(autonomousLearningMarker)
	if strings.Contains(strings.ToLower(trimmed), marker) {
		return trimmed
	}
	return trimmed + "\n\n" + autonomousLearningMarker
}
