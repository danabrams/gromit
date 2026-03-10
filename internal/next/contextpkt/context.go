package contextpkt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danabrams/gromit/internal/next/architecture"
	"github.com/danabrams/gromit/internal/next/doctrine"
	"github.com/danabrams/gromit/internal/next/projectcell"
)

type Level int

const (
	LevelProject Level = iota
	LevelSpec
	LevelTask
)

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
	Compile(ctx context.Context, cell projectcell.Cell, level Level, opts CompileOpts) (Packet, error)
}

type DefaultCompiler struct {
	store ArtifactStore
}

func NewCompiler(store ArtifactStore) *DefaultCompiler {
	return &DefaultCompiler{store: store}
}

func (c *DefaultCompiler) Compile(ctx context.Context, cell projectcell.Cell, level Level, opts CompileOpts) (Packet, error) {
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

	// Load architecture
	var arch architecture.Architecture
	if err := c.store.Read(cell.CellPath, "architecture", &arch); err == nil {
		if len(arch.Modules) > 0 || len(arch.Components) > 0 {
			content, _ := json.MarshalIndent(arch, "", "  ")
			sections = append(sections, Section{
				Name:          "architecture",
				Content:       string(content),
				TokenEstimate: estimateTokens(string(content)),
			})
		}
	}

	// Load doctrine
	var doc doctrine.Doctrine
	if err := c.store.Read(cell.CellPath, "doctrine", &doc); err == nil {
		if len(doc.Rules) > 0 {
			content, _ := json.MarshalIndent(doc, "", "  ")
			sections = append(sections, Section{
				Name:          "doctrine",
				Content:       string(content),
				TokenEstimate: estimateTokens(string(content)),
			})
		}
	}

	// Spec-level: add spec text section
	if level == LevelSpec || level == LevelTask {
		sections = append(sections, Section{
			Name:          "spec-text",
			Content:       fmt.Sprintf("Spec: %s", opts.SpecPath),
			TokenEstimate: estimateTokens(opts.SpecPath),
		})
	}

	// Task-level: add proof requirements section
	if level == LevelTask {
		sections = append(sections, Section{
			Name:          "proof-requirements",
			Content:       fmt.Sprintf("Task %s proof requirements: run tests, check invariants", opts.TaskID),
			TokenEstimate: estimateTokens(opts.TaskID) + 10,
		})
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
