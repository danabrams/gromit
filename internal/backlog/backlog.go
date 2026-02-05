package backlog

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Idea represents a backlog idea
type Idea struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Type      string    `json:"type"`      // feature, bug, chore, unknown
	Context   string    `json:"context"`   // optional additional context
	CreatedAt time.Time `json:"created_at"`
}

// File manages the backlog JSONL file
type File struct {
	path string
}

// NewFile creates a new backlog file manager
func NewFile(ralphDir string) *File {
	return &File{
		path: filepath.Join(ralphDir, "backlog.jsonl"),
	}
}

// Add appends a new idea to the backlog
func (f *File) Add(idea *Idea) error {
	// Ensure .ralph directory exists
	dir := filepath.Dir(f.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening backlog file: %w", err)
	}
	defer file.Close()

	// Write JSON line
	data, err := json.Marshal(idea)
	if err != nil {
		return fmt.Errorf("marshaling idea: %w", err)
	}

	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing to backlog: %w", err)
	}

	return nil
}

// List returns all ideas from the backlog
func (f *File) List() ([]*Idea, error) {
	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Empty backlog
		}
		return nil, fmt.Errorf("opening backlog file: %w", err)
	}
	defer file.Close()

	var ideas []*Idea
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var idea Idea
		if err := json.Unmarshal([]byte(line), &idea); err != nil {
			return nil, fmt.Errorf("parsing backlog line: %w", err)
		}
		ideas = append(ideas, &idea)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading backlog: %w", err)
	}

	return ideas, nil
}

// GenerateID creates a unique ID for an idea
func GenerateID() string {
	return fmt.Sprintf("idea-%d", time.Now().UnixNano()/1000000) // milliseconds
}

// Delete removes an idea from the backlog by rewriting the file without it
func (f *File) Delete(id string) error {
	// Load all ideas
	ideas, err := f.List()
	if err != nil {
		return fmt.Errorf("loading backlog: %w", err)
	}

	// Filter out the idea to delete
	var filtered []*Idea
	found := false
	for _, idea := range ideas {
		if idea.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, idea)
	}

	if !found {
		return fmt.Errorf("idea not found: %s", id)
	}

	// Rewrite the file
	file, err := os.OpenFile(f.path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening backlog file: %w", err)
	}
	defer file.Close()

	for _, idea := range filtered {
		data, err := json.Marshal(idea)
		if err != nil {
			return fmt.Errorf("marshaling idea: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("writing to backlog: %w", err)
		}
	}

	return nil
}

// CategorizeIdea attempts to auto-categorize based on keywords
func CategorizeIdea(text string) string {
	lower := strings.ToLower(text)

	// Check for bug keywords
	bugKeywords := []string{"fix", "bug", "broken", "error", "crash", "issue", "wrong", "fails", "failing"}
	for _, kw := range bugKeywords {
		if strings.Contains(lower, kw) {
			return "bug"
		}
	}

	// Check for feature keywords
	featureKeywords := []string{"add", "implement", "create", "new", "feature", "support", "enable", "allow"}
	for _, kw := range featureKeywords {
		if strings.Contains(lower, kw) {
			return "feature"
		}
	}

	// Check for chore keywords
	choreKeywords := []string{"refactor", "clean", "update", "upgrade", "migrate", "improve", "optimize", "remove"}
	for _, kw := range choreKeywords {
		if strings.Contains(lower, kw) {
			return "chore"
		}
	}

	// Unknown - will need to ask
	return "unknown"
}
