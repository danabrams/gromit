package fact

import (
	"encoding/json"
	"fmt"
	"time"
)

// Category classifies how a fact was obtained.
type Category int

const (
	// Declared facts come from explicit user or config declarations.
	Declared Category = iota
	// Observed facts come from direct observation of the codebase.
	Observed
	// Inferred facts are derived from other facts through reasoning.
	Inferred
)

func (c Category) String() string {
	switch c {
	case Declared:
		return "declared"
	case Observed:
		return "observed"
	case Inferred:
		return "inferred"
	default:
		return "unknown"
	}
}

// MarshalJSON encodes a Category as its string representation.
func (c Category) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// UnmarshalJSON decodes a Category from its string representation.
func (c *Category) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "declared":
		*c = Declared
	case "observed":
		*c = Observed
	case "inferred":
		*c = Inferred
	default:
		return fmt.Errorf("unknown category: %q", s)
	}
	return nil
}

// Fact represents a single piece of knowledge about the codebase.
type Fact struct {
	ID        string    `json:"id"`
	Category  Category  `json:"category"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
}

// New creates a Fact with the given fields and the current timestamp.
func New(id string, cat Category, content string, source string) Fact {
	return Fact{
		ID:        id,
		Category:  cat,
		Content:   content,
		Source:    source,
		Timestamp: time.Now(),
	}
}
