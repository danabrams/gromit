package enrich

import (
	"fmt"
	"time"
)

// IsExpired returns true if the fact is older than the given number of days.
func IsExpired(f InferredFact, expiryDays int) bool {
	expiry := time.Duration(expiryDays) * 24 * time.Hour
	return time.Since(f.CreatedAt) > expiry
}

// FilterExpired returns only non-expired facts.
func FilterExpired(facts []InferredFact, expiryDays int) []InferredFact {
	var result []InferredFact
	for _, f := range facts {
		if !IsExpired(f, expiryDays) {
			result = append(result, f)
		}
	}
	if result == nil {
		result = []InferredFact{}
	}
	return result
}

// CheckObservedFreshness returns a warning string if the observed facts
// provenance SHA doesn't match the current HEAD SHA.
func CheckObservedFreshness(provenanceSHA, headSHA string) string {
	if provenanceSHA != headSHA {
		return fmt.Sprintf("Observed facts are stale (last inspect at %s, HEAD is %s). Run with --refresh or run inspect first.", provenanceSHA, headSHA)
	}
	return ""
}
