package retro

import "time"

// FrictionCluster captures a code area with multiple learnings and supporting evidence.
type FrictionCluster struct {
	Area          string         `json:"area"`
	LearningCount int            `json:"learning_count"`
	EarliestDate  time.Time      `json:"earliest_date"`
	LatestDate    time.Time      `json:"latest_date"`
	Categories    map[string]int `json:"categories"`
}

// FrictionResolution describes how a previously identified friction area evolved.
type FrictionResolution struct {
	Area          string `json:"area"`
	Status        string `json:"status"`
	PreviousCount int    `json:"previous_count"`
	CurrentCount  int    `json:"current_count"`
}
