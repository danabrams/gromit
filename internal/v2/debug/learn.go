package debug

import "strings"

// LearningExtractionInput is the raw learn/recommendation output from debug LLM.
type LearningExtractionInput struct {
	LearningsEntry         string
	SystemicRecommendation string
	RootCause              RootCause
	Diagnosis              *Diagnosis
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

	rootCause, failureSignal, errorText, stageName := learningContextFromDiagnosis(input)

	if systemicRecommendation == "" && isSystemicLearning(learningsEntry) {
		systemicRecommendation = BuildSystemicRecommendation(rootCause, learningsEntry)
		if systemicRecommendation == "" {
			systemicRecommendation = learningsEntry
		}
		learningsEntry = ""
	}
	if systemicRecommendation == "" {
		systemicRecommendation = BuildSystemicRecommendation(rootCause, failureSignal)
	}

	if systemicRecommendation != "" {
		return LearningExtraction{
			SystemicRecommendation: systemicRecommendation,
		}
	}

	if learningsEntry == "" {
		learningsEntry = buildLearningEntryFromDiagnostics(rootCause, failureSignal, errorText, stageName)
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

func learningContextFromDiagnosis(input LearningExtractionInput) (RootCause, string, string, string) {
	rootCause := input.RootCause
	failureSignal := ""
	errorText := ""
	stageName := ""

	if diag := input.Diagnosis; diag != nil {
		if rootCause == "" {
			rootCause = diag.RootCause
		}
		failureSignal = failureSignalFromDiagnosis(diag)
		if trimmed := strings.TrimSpace(diag.StageTrace.FailureMessage); trimmed != "" {
			errorText = trimmed
		} else {
			errorText = failureSignal
		}
		stageName = strings.TrimSpace(diag.StageTrace.StageName)
	}

	return rootCause, failureSignal, errorText, stageName
}

func failureSignalFromDiagnosis(diag *Diagnosis) string {
	if diag == nil {
		return ""
	}
	if trimmed := strings.TrimSpace(diag.StageTrace.FailureMessage); trimmed != "" {
		return trimmed
	}
	if diag.FailureEvent != nil {
		for _, key := range []string{"error", "details", "failed_command"} {
			if raw, ok := diag.FailureEvent[key]; ok {
				if str, ok := raw.(string); ok && strings.TrimSpace(str) != "" {
					return strings.TrimSpace(str)
				}
			}
		}
	}
	return ""
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
