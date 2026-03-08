package debug

import (
	"path/filepath"
	"strings"
)

type learnablePattern struct {
	description string
	example     string
}

var learnablePatterns = map[RootCause]*learnablePattern{
	RootCauseBadBuildOutput: {
		description: "Capture build failure diagnostics before retries so fixes stay grounded in reproducible output.",
		example:     "Commit the Build iter:1 failure log before rerunning so debugging can reference the original diagnostics even after a retry.",
	},
	RootCauseFlakyTest: {
		description: "Treat transient validation failures as flaky signals and rerun validation before changing code.",
		example:     "Re-run `validate` when the error mentions \"flaky\" or \"transient\" so the fix isn't chasing the same ephemeral test failure.",
	},
}

type learnablePatternMatcher struct {
	pattern *learnablePattern
	match   func(RootCause, string, string, string) bool
}

var learnablePatternMatchers = []learnablePatternMatcher{
	{
		pattern: &learnablePattern{
			description: "Capture syntax-error build diagnostics before drifting into unrelated edits so the fix stays tied to the failing files.",
			example:     "When `go build` reports \"syntax error\", rerun `go build ./...` to persist the failing output before editing unrelated code.",
		},
		match: func(_ RootCause, failureSignal, errorText, stage string) bool {
			combined := failureSignal + " " + errorText
			if strings.Contains(combined, "syntax error") {
				return true
			}
			if stage == "build" && strings.Contains(combined, "unexpected") {
				return true
			}
			return false
		},
	},
	{
		pattern: &learnablePattern{
			description: "Re-run lint or validation commands before touching code so transient warnings are confirmed before fixes.",
			example:     "When `go vet` or lint commands report issues, rerun the same command to make sure the failure persists before applying fixes.",
		},
		match: func(_ RootCause, failureSignal, errorText, stage string) bool {
			combined := failureSignal + " " + errorText
			if strings.Contains(combined, "go vet") || strings.Contains(combined, "vet") || strings.Contains(combined, "lint") || strings.Contains(combined, "unused") {
				return true
			}
			if stage == "lint" || stage == "validate" {
				return strings.Contains(combined, "vet") || strings.Contains(combined, "lint")
			}
			return false
		},
	},
}

// identifyLearnablePattern returns a reusable autonomous learning pattern for
// root causes that represent recurring fix conventions.
func identifyLearnablePattern(rootCause RootCause) *learnablePattern {
	return learnablePatterns[rootCause]
}

func buildLearningEntryFromRootCause(rootCause RootCause) string {
	return buildLearningEntryFromDiagnostics(rootCause, "", "", "")
}

func buildLearningEntryFromDiagnostics(rootCause RootCause, failureSignal, errorText, stage string) string {
	pattern := detectLearnablePattern(rootCause, failureSignal, errorText, stage)
	if pattern == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### debug2_learning | CODE_PATTERN\n\n")
	sb.WriteString(pattern.description)
	if pattern.example != "" {
		sb.WriteString("\n\nExample: ")
		sb.WriteString(pattern.example)
	}
	return sb.String()
}

func detectLearnablePattern(rootCause RootCause, failureSignal, errorText, stage string) *learnablePattern {
	normSignal := normalizeDiagnosticText(failureSignal)
	normError := normalizeDiagnosticText(errorText)
	normStage := strings.ToLower(strings.TrimSpace(stage))

	for _, matcher := range learnablePatternMatchers {
		if matcher.match(rootCause, normSignal, normError, normStage) {
			return matcher.pattern
		}
	}
	return identifyLearnablePattern(rootCause)
}

func normalizeDiagnosticText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// PersistLearnablePatternEntry records the learning for the given root cause in the
// spec's LEARNINGS.md file, returning the entry that was written.
func PersistLearnablePatternEntry(specDir string, rootCause RootCause, failureSignal, errorText, stage string) (string, error) {
	trimmedDir := strings.TrimSpace(specDir)
	if trimmedDir == "" {
		return "", ErrEmptyPath
	}

	entry := buildLearningEntryFromDiagnostics(rootCause, failureSignal, errorText, stage)
	if entry == "" {
		return "", nil
	}

	normalized := withAutonomousMarker(entry)
	learningsPath := filepath.Join(trimmedDir, "LEARNINGS.md")
	if err := PersistLearning(learningsPath, normalized); err != nil {
		return "", err
	}
	return normalized, nil
}
