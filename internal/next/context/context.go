// Package context compiles context packets for LLM invocations.
//
// A context packet is a structured bundle of information selected and
// assembled for a specific task. It draws from the agent guide, source map,
// doctrine, and task-specific inputs to produce a token-budgeted payload.
//
// TODO: implement context compilation from artifacts
// TODO: implement token budget allocation across sections
// TODO: implement task-scoped file selection
// TODO: implement context packet serialization
package context

// Packet represents a compiled context packet ready for LLM consumption.
//
// TODO: define full schema (sections, token counts, provenance metadata)
type Packet struct {
	// Sections is the ordered list of context sections.
	Sections []Section `json:"sections"`

	// TotalTokens is the estimated total token count.
	TotalTokens int `json:"total_tokens"`
}

// Section is a single section within a context packet.
type Section struct {
	// Name identifies this section (e.g. "architecture", "relevant_files").
	Name string `json:"name"`

	// Content is the rendered text content.
	Content string `json:"content"`

	// TokenEstimate is the estimated token count for this section.
	TokenEstimate int `json:"token_estimate"`
}

// Builder assembles context packets from workspace artifacts.
//
// TODO: implement builder with configurable section selection and budget
type Builder interface {
	Build(projectID string, taskDescription string) (*Packet, error)
}
