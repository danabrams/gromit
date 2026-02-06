package retro

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/learnings"
)

// Retro manages retrospective analysis
type Retro struct {
	cfg           *config.Config
	claude        *claude.Client
	learningsFile *learnings.File
	rulesPath     string
	templatePath  string
}

// TemplateContext holds data for retro prompt template
type TemplateContext struct {
	Rules     string
	Learnings string
}

// Result represents the outcome of a retro analysis
type Result struct {
	Analysis      string
	ProposedRules string
	Success       bool
}

// NewRetro creates a new retrospective analyzer
func NewRetro(cfg *config.Config, ralphDir string) *Retro {
	if cfg == nil {
		return nil
	}
	return &Retro{
		cfg:           cfg,
		claude:        claude.NewClient(cfg.Claude.Binary, cfg.Claude.Flags, cfg.Claude.Timeout),
		learningsFile: learnings.NewFile(ralphDir),
		rulesPath:     filepath.Join(ralphDir, "RULES.md"),
		templatePath:  filepath.Join(ralphDir, "templates", "PROMPT_retro.md"),
	}
}

// Run executes the retrospective analysis
func (r *Retro) Run(ctx context.Context, apply bool) (*Result, error) {
	if r == nil {
		return nil, fmt.Errorf("retro is nil")
	}
	// Load learnings
	if err := r.learningsFile.Load(); err != nil {
		return nil, fmt.Errorf("loading learnings: %w", err)
	}

	// Load rules
	rules, err := r.loadRules()
	if err != nil {
		return nil, fmt.Errorf("loading rules: %w", err)
	}

	// Format learnings for prompt
	learningsText := r.formatLearnings()

	// Render prompt
	prompt, err := r.renderPrompt(rules, learningsText)
	if err != nil {
		return nil, fmt.Errorf("rendering prompt: %w", err)
	}

	// Run Claude analysis (use opus for quality analysis)
	model := "opus"
	claudeResult, err := r.claude.Run(ctx, prompt, model)
	if err != nil {
		return nil, fmt.Errorf("running Claude analysis: %w", err)
	}

	if claudeResult == nil {
		return &Result{Success: false}, nil
	}

	result := &Result{
		Analysis: claudeResult.Output,
		Success:  claudeResult.Success,
	}

	// If apply flag is set, extract and apply changes
	if apply && claudeResult.Success {
		if err := r.applyChanges(claudeResult.Output); err != nil {
			return nil, fmt.Errorf("applying changes: %w", err)
		}
	}

	return result, nil
}

// loadRules reads the RULES.md file
func (r *Retro) loadRules() (string, error) {
	content, err := os.ReadFile(r.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading rules file: %w", err)
	}
	return string(content), nil
}

// formatLearnings formats learnings into a readable string
func (r *Retro) formatLearnings() string {
	var sb strings.Builder

	confirmed := r.learningsFile.GetConfirmed()
	provisional := r.learningsFile.GetProvisional()

	sb.WriteString("## Confirmed Learnings\n\n")
	if len(confirmed) == 0 {
		sb.WriteString("*No confirmed learnings yet.*\n\n")
	} else {
		for _, l := range confirmed {
			sb.WriteString(fmt.Sprintf("**%s | %s | %s**\n",
				l.Date.Format("2006-01-02"),
				l.BeadID,
				l.Category,
			))
			if l.RelatedTo != "" {
				sb.WriteString(fmt.Sprintf("*Related to: %s*\n", l.RelatedTo))
			}
			sb.WriteString(l.Content)
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("## Provisional Learnings\n\n")
	if len(provisional) == 0 {
		sb.WriteString("*No provisional learnings.*\n\n")
	} else {
		for _, l := range provisional {
			sb.WriteString(fmt.Sprintf("**%s | %s | %s**\n",
				l.Date.Format("2006-01-02"),
				l.BeadID,
				l.Category,
			))
			if l.RelatedTo != "" {
				sb.WriteString(fmt.Sprintf("*Related to: %s*\n", l.RelatedTo))
			}
			sb.WriteString(l.Content)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// renderPrompt renders the retro prompt template
func (r *Retro) renderPrompt(rules, learnings string) (string, error) {
	tmplContent, err := os.ReadFile(r.templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("retro").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	ctx := TemplateContext{
		Rules:     rules,
		Learnings: learnings,
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

// applyChanges parses the analysis output and applies changes to RULES.md
func (r *Retro) applyChanges(analysis string) error {
	// For now, this is a placeholder
	// In a full implementation, we would:
	// 1. Parse the analysis output to extract proposed changes
	// 2. Apply them to RULES.md
	// 3. Update LEARNINGS.md to remove archived items
	//
	// This requires more sophisticated parsing logic which
	// would be better handled by having Claude output structured
	// data (e.g., JSON) or by doing interactive confirmation

	// Write the analysis to a file for manual review
	analysisPath := filepath.Join(filepath.Dir(r.rulesPath), "RETRO_PROPOSED_CHANGES.md")
	if err := os.WriteFile(analysisPath, []byte(analysis), 0644); err != nil {
		return fmt.Errorf("writing proposed changes: %w", err)
	}

	return fmt.Errorf("auto-apply not yet implemented - review %s and apply manually", analysisPath)
}
