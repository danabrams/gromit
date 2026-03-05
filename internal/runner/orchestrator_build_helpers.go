package runner

import (
	"os/exec"
	"strings"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/pipeline"
)

func stampBuildAttribution(log *logger.IterationLog, buildOut pipeline.Output) {
	if log == nil {
		return
	}
	log.Model = buildOut.Model
	log.CostUSD = buildOut.CostUSD
	log.InputTokens = buildOut.InputTokens
	log.OutputTokens = buildOut.OutputTokens
	log.DurationMs = buildOut.DurationMs
	log.OriginalTier = buildOut.OriginalTier
	log.ActualTier = buildOut.ActualTier
	log.CacheHit = buildOut.CacheHit
	log.CacheMiss = buildOut.CacheMiss
	log.CacheWrite = buildOut.CacheWrite
	log.CacheClass = buildOut.CacheClass
	log.CacheKey = buildOut.CacheKey
	log.CacheInvalidationReason = buildOut.CacheInvalidationReason
	log.CacheVersionMarker = buildOut.CacheVersionMarker
}

// countChangedFiles returns the number of files changed since startCommit
// by running git diff --name-only. Returns 0 if startCommit is empty or
// the command fails.
func countChangedFiles(startCommit string) int {
	if startCommit == "" {
		return 0
	}
	cmd := exec.Command("git", "diff", "--name-only", startCommit)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return 0
	}
	return len(lines)
}

func inferBuildFailurePhase(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "red phase"):
		return "red"
	case strings.Contains(msg, "green phase"):
		return "green"
	case strings.Contains(msg, "refactor phase"):
		return "refactor"
	case strings.Contains(msg, "final validation"):
		return "final_validation"
	default:
		return "build"
	}
}
