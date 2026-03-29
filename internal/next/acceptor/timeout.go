package acceptor

import (
	"strings"
	"time"
)

// TimeoutConfig holds the constants that drive per-criterion timeout scaling.
type TimeoutConfig struct {
	BaseSeconds         int `json:"base_seconds" yaml:"base_seconds"`
	RateConstant        int `json:"rate_constant" yaml:"rate_constant"`
	ComplexityBonusSecs int `json:"complexity_bonus_seconds" yaml:"complexity_bonus_seconds"`
	HardMaximumSecs     int `json:"hard_maximum_seconds" yaml:"hard_maximum_seconds"`
}

// DefaultTimeoutConfig returns the production-ready timeout constants.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		BaseSeconds:         60,
		RateConstant:        5000,
		ComplexityBonusSecs: 120,
		HardMaximumSecs:     600,
	}
}

const (
	complexitySimple  = "simple"
	complexityComplex = "complex"
)

var complexityKeywords = []string{
	"end-to-end",
	"pipeline",
	"integration",
	"behavior",
	"scenario",
	"workflow",
	"sequence",
	"survive",
	"resume",
}

// ClassifyCriterionComplexity returns a keyword-based classification for the
// provided criterion text.
func ClassifyCriterionComplexity(criterion string) string {
	lower := strings.ToLower(criterion)
	for _, keyword := range complexityKeywords {
		if strings.Contains(lower, keyword) {
			return complexityComplex
		}
	}
	return complexitySimple
}

// ComputeCriterionTimeout returns the context timeout for evaluating a single
// criterion using the configured formula.
func ComputeCriterionTimeout(cfg TimeoutConfig, diffSizeBytes int, criterion string) time.Duration {
	baseSecs := cfg.BaseSeconds
	if baseSecs <= 0 {
		baseSecs = 1
	}

	rate := cfg.RateConstant
	if rate <= 0 {
		rate = 1
	}

	totalSeconds := float64(baseSecs)
	totalSeconds += float64(diffSizeBytes) / float64(rate)

	if ClassifyCriterionComplexity(criterion) == complexityComplex {
		totalSeconds += float64(cfg.ComplexityBonusSecs)
	}

	if totalSeconds < float64(baseSecs) {
		totalSeconds = float64(baseSecs)
	}

	if cfg.HardMaximumSecs > 0 && totalSeconds > float64(cfg.HardMaximumSecs) {
		totalSeconds = float64(cfg.HardMaximumSecs)
	}

	return time.Duration(totalSeconds * float64(time.Second))
}
