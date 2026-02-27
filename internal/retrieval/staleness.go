package retrieval

import (
	"fmt"
	"time"
)

// StalenessPolicy defines how index freshness is evaluated.
type StalenessPolicy string

const (
	StalenessPolicyNone   StalenessPolicy = "none"
	StalenessPolicyMaxAge StalenessPolicy = "max_age"
)

// StalenessDetector evaluates whether an index is stale based on policy.
type StalenessDetector struct {
	now func() time.Time
}

// NewStalenessDetector returns a detector using the provided clock.
func NewStalenessDetector(now func() time.Time) *StalenessDetector {
	if now == nil {
		now = time.Now
	}
	return &StalenessDetector{now: now}
}

// Check returns (isStale, reason).
func (d *StalenessDetector) Check(metadata IndexMetadata, policy StalenessPolicy, thresholdDays int) (bool, string) {
	if policy != StalenessPolicyMaxAge || thresholdDays <= 0 {
		return false, ""
	}

	age := d.now().Sub(metadata.LastUpdated)
	threshold := time.Duration(thresholdDays) * 24 * time.Hour
	if age > threshold {
		return true, fmt.Sprintf("index stale (age %s > %s)", age.Round(time.Second), threshold)
	}

	return false, ""
}
