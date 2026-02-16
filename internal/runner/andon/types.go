package andon

import "time"

// AndonLevel identifies the escalation depth.
type AndonLevel string

const (
	LevelL1 AndonLevel = "L1"
	LevelL2 AndonLevel = "L2"
	LevelL3 AndonLevel = "L3"
	LevelL4 AndonLevel = "L4"
)

// AndonThresholds defines policy limits for autonomous recovery.
type AndonThresholds struct {
	L1MaxRetries   int
	L1MaxDuration  time.Duration
	L2MaxDuration  time.Duration
	MaxAssumptions int
}
