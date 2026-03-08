package debug

import "strings"

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

// identifyLearnablePattern returns a reusable autonomous learning pattern for
// root causes that represent recurring fix conventions.
func identifyLearnablePattern(rootCause RootCause) *learnablePattern {
	return learnablePatterns[rootCause]
}

func buildLearningEntryFromRootCause(rootCause RootCause) string {
	pattern := identifyLearnablePattern(rootCause)
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
