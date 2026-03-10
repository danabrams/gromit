package contextpkt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/doctrine"
)

type Level int

const (
	LevelProject Level = iota
	LevelSpec
	LevelTask
)

// Cell is the local type used by the Compiler. This decouples contextpkt
// from projectcell so callers can construct a Cell without importing that package.
type Cell struct {
	Name     string
	CellPath string
}

type CompileOpts struct {
	SpecPath    string
	TaskID      string
	TokenBudget int
}

type Packet struct {
	Level      Level     `json:"level"`
	Sections   []Section `json:"sections"`
	TokenCount int       `json:"token_count"`
}

// NormalizeNilFields maps nil slices to empty values.
func (p *Packet) NormalizeNilFields() {
	if p.Sections == nil {
		p.Sections = []Section{}
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

	sections = append(sections, Section{
		Name:          "glossary",
		Content:       "Project glossary placeholder",
		TokenEstimate: estimateTokens("Project glossary placeholder"),
	})
	sections = append(sections, Section{
		Name:          "validation",
		Content:       "Project validation rules placeholder",
		TokenEstimate: estimateTokens("Project validation rules placeholder"),
	})

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
	var arch architecture.Architecture
	if err := c.store.Read(cell.CellPath, "architecture", &arch); err != nil {
		return Section{}, false
	}
	if len(arch.Modules) == 0 && len(arch.Components) == 0 {
		return Section{}, false
	}
	content, err := json.MarshalIndent(arch, "", "  ")
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
	var doc doctrine.Doctrine
	if err := c.store.Read(cell.CellPath, "doctrine", &doc); err != nil {
		return Section{}, false
	}
	if len(doc.Rules) == 0 {
		return Section{}, false
	}
	content, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Section{}, false
	}
	return Section{
		Name:          "doctrine",
		Content:       string(content),
		TokenEstimate: estimateTokens(string(content)),
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
	var result []Section
	remaining := budget
	for _, s := range sections {
		if s.TokenEstimate <= remaining {
			result = append(result, s)
			remaining -= s.TokenEstimate
		} else if remaining > 0 {
			// Truncate content to fit
			chars := remaining * 4
			if chars > len(s.Content) {
				chars = len(s.Content)
			}
			result = append(result, Section{
				Name:          s.Name,
				Content:       s.Content[:chars],
				TokenEstimate: remaining,
				Facts:         s.Facts,
			})
			remaining = 0
		}
	}
	return result
}
