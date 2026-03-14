package acceptor

// ParseError represents an error parsing LLM output (unparseable JSON, missing fields, invalid values).
type ParseError struct {
	Msg string
}

func (e *ParseError) Error() string { return e.Msg }
