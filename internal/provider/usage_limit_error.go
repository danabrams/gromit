package provider

import "fmt"

// UsageLimitError represents an invocation that failed because a usage limit was exceeded.
type UsageLimitError struct {
	Type    string
	Message string
}

// Error implements the error interface.
func (e *UsageLimitError) Error() string {
	if e == nil {
		return "usage limit exceeded"
	}
	switch {
	case e.Type != "" && e.Message != "":
		return fmt.Sprintf("%s: %s", e.Type, e.Message)
	case e.Message != "":
		return e.Message
	case e.Type != "":
		return e.Type
	default:
		return "usage limit exceeded"
	}
}
