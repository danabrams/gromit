package debug

import "fmt"

// identifyLearnablePattern returns a reusable autonomous learning pattern for
// root causes that represent recurring fix conventions.
func identifyLearnablePattern(rootCause RootCause) string {
	switch rootCause {
	case RootCauseBadBuildOutput:
		return "Capture build failure diagnostics before retries so fixes stay grounded in reproducible output."
	case RootCauseFlakyTest:
		return "Treat transient validation failures as flaky signals and rerun validation before changing code."
	default:
		return ""
	}
}

func buildLearningEntryFromRootCause(rootCause RootCause) string {
	pattern := identifyLearnablePattern(rootCause)
	if pattern == "" {
		return ""
	}
	return fmt.Sprintf("### debug2_learning | CODE_PATTERN\n\n%s", pattern)
}
