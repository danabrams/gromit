package debug

// FailureContext captures information about a failed stage for pattern detection.
type FailureContext struct {
	Message string
	Stage   string
	BeadID  string
}

// LearningPattern describes a known pattern that can be fixed autonomously.
type LearningPattern struct {
	ID              string
	Category        string
	BeadID          string
	LearningContent string
	FixPlan         FixPlan
	Trigger         func(FailureContext) bool
}

// DetectLearnablePattern returns the first pattern whose trigger matches the context.
func DetectLearnablePattern(ctx FailureContext, patterns []LearningPattern) *LearningPattern {
	for i := range patterns {
		if patterns[i].Trigger == nil {
			continue
		}
		if patterns[i].Trigger(ctx) {
			return &patterns[i]
		}
	}
	return nil
}
