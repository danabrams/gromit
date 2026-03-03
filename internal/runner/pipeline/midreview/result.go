package midreview

// Finding represents a single finding from the mid-build review.
type Finding struct {
	Category string `json:"category"`
	Message  string `json:"message"`
}
