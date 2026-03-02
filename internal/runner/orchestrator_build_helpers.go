package runner

import (
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
