package retro

// FrictionResolution describes how a previously identified friction area evolved.
type FrictionResolution struct {
	Area          string `json:"area"`
	Status        string `json:"status"`
	PreviousCount int    `json:"previous_count"`
	CurrentCount  int    `json:"current_count"`
}
