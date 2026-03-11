package contextpkt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Local types for deserializing artifacts without importing concrete packages.
type archData struct {
	Modules    []json.RawMessage `json:"modules"`
	Components []json.RawMessage `json:"components"`
}

type doctrineData struct {
	Rules []json.RawMessage `json:"rules"`
}

type Level int

const (
	LevelProject Level = iota
	LevelSpec
	LevelTask
)

func (l Level) String() string {
	switch l {
	case LevelProject:
		return "project"
	case LevelSpec:
		return "spec"
	case LevelTask:
		return "task"
	default:
		return "unknown"
	}
}

func (l Level) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.String())
}

func (l *Level) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "project":
		*l = LevelProject
	case "spec":
		*l = LevelSpec
	case "task":
		*l = LevelTask
	default:
		return fmt.Errorf("unknown level: %q", s)
	}
	return nil
}

// Cell is the local type used by the Compiler. This decouples contextpkt
// from projectcell so callers can construct a Cell without importing that package.
type Cell struct {
	Name     string
	CellPath string
}

type CompileOpts struct {
	SpecPath            string
	TaskID              string
	TokenBudget         int
	IncludeInferred     bool
	StalenessExpiryDays int
}

type Packet struct {
	Level      Level     `json:"level"`
	Sections   []Section `json:"sections"`
	TokenCount int       `json:"token_count"`
}

// NormalizeNilFields maps nil slices/maps to empty values.
// See CLAUDE.md nil-field normalization visibility convention.
func (p *Packet) NormalizeNilFields() {
	if p.Sections == nil {
		p.Sections = []Section{}
	}
	for i := range p.Sections {
		if p.Sections[i].Facts == nil {
			p.Sections[i].Facts = []FactRef{}
		}
	}
}

type Section struct {
	Name          string    `json:"name"`
	Content       string    `json:"content"`
	TokenEstimate int       `json:"token_estimate"`
	Facts         []FactRef `json:"facts,omitempty"`
}

type FactRef struct {
	FactID   string `json:"fact_id"`
	Category string `json:"category"`
}

type ArtifactStore interface {
	Read(cellPath string, artifact string, dest any) error
	Write(cellPath string, artifact string, src any) error
	Exists(cellPath string, artifact string) bool
}

type Compiler interface {
	Compile(ctx context.Context, cell Cell, level Level, opts CompileOpts) (Packet, error)
}

type DefaultCompiler struct {
	store ArtifactStore
}

func NewCompiler(store ArtifactStore) *DefaultCompiler {
	return &DefaultCompiler{store: store}
}

func (c *DefaultCompiler) Compile(ctx context.Context, cell Cell, level Level, opts CompileOpts) (Packet, error) {
	switch level {
	case LevelSpec:
		if opts.SpecPath == "" {
			return Packet{}, fmt.Errorf("spec path is required for spec level")
		}
	case LevelTask:
		if opts.SpecPath == "" {
			return Packet{}, fmt.Errorf("spec path is required for task level")
		}
		if opts.TaskID == "" {
			return Packet{}, fmt.Errorf("task ID is required for task level")
		}
	}

	var sections []Section

	switch level {
	case LevelProject:
		sections = c.buildProjectSections(cell)
	case LevelSpec:
		sections = c.buildSpecSections(cell, opts)
	case LevelTask:
		sections = c.buildTaskSections(cell, opts)
	}

	// Append inferred observations if opted in.
	if opts.IncludeInferred {
		expiryDays := opts.StalenessExpiryDays
		if expiryDays == 0 {
			expiryDays = 30
		}
		if s, ok := inferredSection(cell, expiryDays); ok {
			sections = append(sections, s)
		}
	}

	// Calculate total tokens
	totalTokens := 0
	for _, s := range sections {
		totalTokens += s.TokenEstimate
	}

	// Apply token budget if set
	if opts.TokenBudget > 0 && totalTokens > opts.TokenBudget {
		sections = trimToBudget(sections, opts.TokenBudget)
		totalTokens = 0
		for _, s := range sections {
			totalTokens += s.TokenEstimate
		}
	}

	return Packet{
		Level:      level,
		Sections:   sections,
		TokenCount: totalTokens,
	}, nil
}

// buildProjectSections returns: architecture (full) + doctrine (full) + glossary + validation.
func (c *DefaultCompiler) buildProjectSections(cell Cell) []Section {
	var sections []Section

	if s, ok := c.architectureSection(cell); ok {
		sections = append(sections, s)
	}
	if s, ok := c.doctrineSection(cell); ok {
		sections = append(sections, s)
	}

	if s, ok := c.glossarySection(cell); ok {
		sections = append(sections, s)
	}
	if s, ok := c.validationSection(cell); ok {
		sections = append(sections, s)
	}

	return sections
}

// buildSpecSections returns: architecture + doctrine + spec-text.
func (c *DefaultCompiler) buildSpecSections(cell Cell, opts CompileOpts) []Section {
	var sections []Section

	if s, ok := c.architectureSection(cell); ok {
		sections = append(sections, s)
	}
	if s, ok := c.doctrineSection(cell); ok {
		sections = append(sections, s)
	}

	sections = append(sections, Section{
		Name:          "spec-text",
		Content:       fmt.Sprintf("Spec: %s", opts.SpecPath),
		TokenEstimate: estimateTokens(opts.SpecPath),
	})

	return sections
}

// buildTaskSections returns: doctrine + spec-text + proof-requirements.
func (c *DefaultCompiler) buildTaskSections(cell Cell, opts CompileOpts) []Section {
	var sections []Section

	if s, ok := c.doctrineSection(cell); ok {
		sections = append(sections, s)
	}

	sections = append(sections, Section{
		Name:          "spec-text",
		Content:       fmt.Sprintf("Spec: %s", opts.SpecPath),
		TokenEstimate: estimateTokens(opts.SpecPath),
	})

	sections = append(sections, Section{
		Name:          "proof-requirements",
		Content:       fmt.Sprintf("Task %s proof requirements: run tests, check invariants", opts.TaskID),
		TokenEstimate: estimateTokens(opts.TaskID) + 10,
	})

	return sections
}

// architectureSection loads and marshals the architecture artifact.
func (c *DefaultCompiler) architectureSection(cell Cell) (Section, bool) {
	var raw json.RawMessage
	if err := c.store.Read(cell.CellPath, "architecture", &raw); err != nil {
		return Section{}, false
	}
	var data archData
	if err := json.Unmarshal(raw, &data); err != nil {
		return Section{}, false
	}
	if len(data.Modules) == 0 && len(data.Components) == 0 {
		return Section{}, false
	}
	// Re-indent the raw JSON for consistent formatting.
	var indented json.RawMessage
	if err := json.Unmarshal(raw, &indented); err != nil {
		return Section{}, false
	}
	content, err := json.MarshalIndent(indented, "", "  ")
	if err != nil {
		return Section{}, false
	}
	return Section{
		Name:          "architecture",
		Content:       string(content),
		TokenEstimate: estimateTokens(string(content)),
	}, true
}

// doctrineSection loads and marshals the doctrine artifact.
func (c *DefaultCompiler) doctrineSection(cell Cell) (Section, bool) {
	var raw json.RawMessage
	if err := c.store.Read(cell.CellPath, "doctrine", &raw); err != nil {
		return Section{}, false
	}
	var data doctrineData
	if err := json.Unmarshal(raw, &data); err != nil {
		return Section{}, false
	}
	if len(data.Rules) == 0 {
		return Section{}, false
	}
	// Re-indent the raw JSON for consistent formatting.
	var indented json.RawMessage
	if err := json.Unmarshal(raw, &indented); err != nil {
		return Section{}, false
	}
	content, err := json.MarshalIndent(indented, "", "  ")
	if err != nil {
		return Section{}, false
	}
	return Section{
		Name:          "doctrine",
		Content:       string(content),
		TokenEstimate: estimateTokens(string(content)),
	}, true
}

// glossarySection loads the glossary artifact if it exists.
func (c *DefaultCompiler) glossarySection(cell Cell) (Section, bool) {
	if !c.store.Exists(cell.CellPath, "glossary") {
		return Section{}, false
	}
	var raw json.RawMessage
	if err := c.store.Read(cell.CellPath, "glossary", &raw); err != nil {
		return Section{}, false
	}
	content, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return Section{}, false
	}
	return Section{
		Name:          "glossary",
		Content:       string(content),
		TokenEstimate: estimateTokens(string(content)),
	}, true
}

// validationSection loads the validation artifact if it exists.
func (c *DefaultCompiler) validationSection(cell Cell) (Section, bool) {
	if !c.store.Exists(cell.CellPath, "validation") {
		return Section{}, false
	}
	var raw json.RawMessage
	if err := c.store.Read(cell.CellPath, "validation", &raw); err != nil {
		return Section{}, false
	}
	content, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return Section{}, false
	}
	return Section{
		Name:          "validation",
		Content:       string(content),
		TokenEstimate: estimateTokens(string(content)),
	}, true
}

// inferredFact represents a single inferred fact from facts.json.
type inferredFact struct {
	FactID     string    `json:"fact_id"`
	Category   string    `json:"category"`
	Statement  string    `json:"statement"`
	Confidence string    `json:"confidence"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// inferredSection reads inferred/facts.json from the cell and builds a section.
// It excludes facts with status "superseded" or "rejected", and facts older
// than expiryDays. Returns (section, true) if active facts exist, or
// (Section{}, false) if the file is missing, empty, or all facts are filtered out.
func inferredSection(cell Cell, expiryDays int) (Section, bool) {
	path := filepath.Join(cell.CellPath, "inferred", "facts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Section{}, false
	}

	var allFacts []inferredFact
	if err := json.Unmarshal(data, &allFacts); err != nil {
		return Section{}, false
	}

	expiry := time.Duration(expiryDays) * 24 * time.Hour
	now := time.Now()

	var facts []inferredFact
	for _, f := range allFacts {
		if f.Status == "superseded" || f.Status == "rejected" {
			continue
		}
		if !f.CreatedAt.IsZero() && now.Sub(f.CreatedAt) > expiry {
			continue
		}
		facts = append(facts, f)
	}

	if len(facts) == 0 {
		return Section{}, false
	}

	var sb strings.Builder
	sb.WriteString("[INFERRED]\n")
	for _, f := range facts {
		fmt.Fprintf(&sb, "- %s (confidence: %s)\n", f.Statement, f.Confidence)
	}
	content := sb.String()

	refs := make([]FactRef, len(facts))
	for i, f := range facts {
		refs[i] = FactRef{FactID: f.FactID, Category: "inferred"}
	}

	return Section{
		Name:          "inferred-observations",
		Content:       content,
		TokenEstimate: estimateTokens(content),
		Facts:         refs,
	}, true
}

func estimateTokens(s string) int {
	// Rough estimate: ~4 chars per token
	tokens := len(s) / 4
	if tokens == 0 && len(s) > 0 {
		tokens = 1
	}
	return tokens
}

func trimToBudget(sections []Section, budget int) []Section {
	result := []Section{}
	remaining := budget
	for _, s := range sections {
		if s.TokenEstimate <= remaining {
			result = append(result, s)
			remaining -= s.TokenEstimate
		} else if remaining > 0 {
			// Truncate content to fit
			chars := remaining * 4
			content := s.Content
			if chars > len(content) {
				chars = len(content)
			}
			// Back up to a valid UTF-8 rune boundary
			for chars > 0 && chars < len(content) && !utf8.RuneStart(content[chars]) {
				chars--
			}
			result = append(result, Section{
				Name:          s.Name,
				Content:       content[:chars],
				TokenEstimate: remaining,
				Facts:         s.Facts,
			})
			remaining = 0
		}
	}
	return result
}
