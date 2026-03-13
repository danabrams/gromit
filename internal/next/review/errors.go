package review

// ParseError represents an error parsing LLM output (unparseable JSON, missing fields).
// The runner retries on ParseError; other errors fail the facet immediately.
type ParseError struct {
	Msg string
}

func (e *ParseError) Error() string { return e.Msg }
