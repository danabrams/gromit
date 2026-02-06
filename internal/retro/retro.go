package retro

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/claude"
	"github.com/danabrams/ralph-runner/internal/config"
	"github.com/danabrams/ralph-runner/internal/learnings"
	"github.com/danabrams/ralph-runner/internal/logger"
	"github.com/danabrams/ralph-runner/internal/rules"
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
	Rules       string
	Learnings   string
	RunStats    logger.RunStats
	BeadStats   map[string]logger.BeadStats
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
	if r.claude == nil {
		return nil, fmt.Errorf("claude client is nil")
	}
	// Load learnings
	if r.learningsFile == nil {
		return nil, fmt.Errorf("learnings file is nil")
	}
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

	// Load run stats and per-bead stats
	logsDir := filepath.Join(filepath.Dir(r.rulesPath), "logs")
	runStats, _ := logger.ReadAllLogs(logsDir)
	allBeadStats, _ := logger.ReadPerBeadStats(logsDir)

	// Filter per-bead stats to only include beads with >= 2 failures
	filteredBeadStats := make(map[string]logger.BeadStats)
	for id, stats := range allBeadStats {
		if stats.Failures >= 2 {
			filteredBeadStats[id] = stats
		}
	}

	// Render prompt
	prompt, err := r.renderPrompt(rules, learningsText, runStats, filteredBeadStats)
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
			sb.WriteString(fmt.Sprintf("**%s | %s | %s | Hash: `%s`**\n",
				l.Date.Format("2006-01-02"),
				l.BeadID,
				l.Category,
				l.Hash,
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
			sb.WriteString(fmt.Sprintf("**%s | %s | %s | Hash: `%s`**\n",
				l.Date.Format("2006-01-02"),
				l.BeadID,
				l.Category,
				l.Hash,
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
func (r *Retro) renderPrompt(rules, learnings string, runStats logger.RunStats, beadStats map[string]logger.BeadStats) (string, error) {
	tmplContent, err := os.ReadFile(r.templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("retro").Funcs(template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
	}).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	ctx := TemplateContext{
		Rules:     rules,
		Learnings: learnings,
		RunStats:  runStats,
		BeadStats: beadStats,
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, ctx); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

// applyChanges parses the analysis output and applies changes to RULES.md
func (r *Retro) applyChanges(analysis string) error {
	// Parse proposals from the analysis output
	proposals, err := ParseProposals(analysis)
	if err != nil {
		// Write the analysis to a file for manual review if parsing fails
		analysisPath := filepath.Join(filepath.Dir(r.rulesPath), "RETRO_PROPOSED_CHANGES.md")
		if writeErr := os.WriteFile(analysisPath, []byte(analysis), 0644); writeErr != nil {
			return fmt.Errorf("parsing proposals failed: %w; additionally failed to write analysis: %v", err, writeErr)
		}
		return fmt.Errorf("parsing proposals: %w (analysis written to %s for manual review)", err, analysisPath)
	}

	// Convert Proposals to AcceptedProposals (all proposals are accepted in auto-apply mode)
	accepted := &AcceptedProposals{
		Consolidations: proposals.Consolidations,
		Promotions:     proposals.Promotions,
		Archives:       proposals.Archives,
		RuleChanges:    proposals.RuleChanges,
	}

	// Apply accepted proposals
	return r.ApplyAccepted(accepted)
}

// ApplyAccepted executes all changes in the accepted proposals
// Each operation is independent; failures are logged as warnings and do not stop execution
func (r *Retro) ApplyAccepted(accepted *AcceptedProposals) error {
	if accepted == nil {
		return fmt.Errorf("accepted proposals is nil")
	}

	var errors []string

	// Apply consolidations
	for i, c := range accepted.Consolidations {
		if err := r.applyConsolidation(c); err != nil {
			log.Printf("Warning: consolidation %d/%d failed: %v", i+1, len(accepted.Consolidations), err)
			errors = append(errors, fmt.Sprintf("consolidation %d: %v", i+1, err))
		}
	}

	// Apply archives
	for i, a := range accepted.Archives {
		if err := r.applyArchive(a); err != nil {
			log.Printf("Warning: archive %d/%d failed: %v", i+1, len(accepted.Archives), err)
			errors = append(errors, fmt.Sprintf("archive %d: %v", i+1, err))
		}
	}

	// Apply promotions
	for i, p := range accepted.Promotions {
		if err := r.applyPromotion(p); err != nil {
			log.Printf("Warning: promotion %d/%d failed: %v", i+1, len(accepted.Promotions), err)
			errors = append(errors, fmt.Sprintf("promotion %d: %v", i+1, err))
		}
	}

	// Apply rule changes
	for i, rc := range accepted.RuleChanges {
		if err := r.applyRuleChange(rc); err != nil {
			log.Printf("Warning: rule change %d/%d failed: %v", i+1, len(accepted.RuleChanges), err)
			errors = append(errors, fmt.Sprintf("rule change %d: %v", i+1, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("some operations failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// applyConsolidation merges multiple learnings into one
func (r *Retro) applyConsolidation(c ConsolidationProposal) error {
	if len(c.LearningHashes) == 0 {
		return fmt.Errorf("no learning hashes provided")
	}
	if c.ConsolidatedText == "" {
		return fmt.Errorf("no consolidated text provided")
	}

	// Determine category from the first learning
	var category string
	for _, hash := range c.LearningHashes {
		learning := r.learningsFile.GetByHash(hash)
		if learning != nil {
			category = learning.Category
			break
		}
	}
	if category == "" {
		category = learnings.CategoryPatterns // Default fallback
	}

	// Replace old learnings with consolidated version
	if err := r.learningsFile.Replace(c.LearningHashes, c.ConsolidatedText, category); err != nil {
		return fmt.Errorf("replacing learnings: %w", err)
	}

	return nil
}

// applyArchive moves a learning to the archived section
func (r *Retro) applyArchive(a ArchiveProposal) error {
	if a.LearningHash == "" {
		return fmt.Errorf("no learning hash provided")
	}

	if err := r.learningsFile.Archive(a.LearningHash, a.Rationale); err != nil {
		return fmt.Errorf("archiving learning: %w", err)
	}

	return nil
}

// applyPromotion promotes a learning to a rule
func (r *Retro) applyPromotion(p PromotionProposal) error {
	if p.LearningHash == "" {
		return fmt.Errorf("no learning hash provided")
	}
	if p.ProposedRule == "" {
		return fmt.Errorf("no proposed rule text provided")
	}
	if p.Section == "" {
		return fmt.Errorf("no section specified")
	}

	// Load current rules
	rulesData, err := rules.Load(r.rulesPath)
	if err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}

	// Add the new rule to the specified section
	rulesData.AddRule(p.Section, p.ProposedRule)

	// Save updated rules
	if err := rulesData.Save(r.rulesPath); err != nil {
		return fmt.Errorf("saving rules: %w", err)
	}

	// Archive the promoted learning
	learning := r.learningsFile.GetByHash(p.LearningHash)
	if learning != nil {
		reason := fmt.Sprintf("promoted to rule in section %s", p.Section)
		if err := r.learningsFile.Archive(p.LearningHash, reason); err != nil {
			// Log warning but don't fail - the rule was already added
			log.Printf("Warning: failed to archive promoted learning %s: %v", p.LearningHash, err)
		}
	}

	return nil
}

// applyRuleChange modifies an existing rule
func (r *Retro) applyRuleChange(rc RuleChangeProposal) error {
	if rc.CurrentRule == "" {
		return fmt.Errorf("no current rule text provided")
	}
	if rc.ProposedRule == "" {
		return fmt.Errorf("no proposed rule text provided")
	}

	// Load current rules
	rulesData, err := rules.Load(r.rulesPath)
	if err != nil {
		return fmt.Errorf("loading rules: %w", err)
	}

	// Try to find and modify the rule in each section
	var modified bool
	var lastErr error
	for _, section := range rulesData.Sections {
		if err := rulesData.ModifyRule(section.Name, rc.CurrentRule, rc.ProposedRule); err == nil {
			modified = true
			break
		} else {
			lastErr = err
		}
	}

	if !modified {
		if lastErr != nil {
			return fmt.Errorf("rule not found in any section: %w", lastErr)
		}
		return fmt.Errorf("rule not found: %q", rc.CurrentRule)
	}

	// Save updated rules
	if err := rulesData.Save(r.rulesPath); err != nil {
		return fmt.Errorf("saving rules: %w", err)
	}

	return nil
}

// enrichBeadStats populates Status, CloseReason, and Comments fields on BeadStats
// by calling bd show for each bead. Errors are logged as warnings and do not stop enrichment.
func (r *Retro) enrichBeadStats(ctx context.Context, beadStats map[string]logger.BeadStats) {
	if r == nil || beadStats == nil {
		return
	}

	client := bead.NewClient()
	if client == nil {
		log.Printf("Warning: failed to create bead client for enrichment")
		return
	}

	for beadID, stats := range beadStats {
		// Get full bead details
		b, err := client.Show(beadID)
		if err != nil {
			log.Printf("Warning: failed to get details for bead %s: %v", beadID, err)
			continue
		}

		// Populate status and close reason
		stats.Status = b.Status
		stats.CloseReason = b.CloseReason

		// Get comments
		comments, err := client.GetComments(beadID)
		if err != nil {
			log.Printf("Warning: failed to get comments for bead %s: %v", beadID, err)
			// Continue with status/close_reason populated
		} else {
			// Extract comment text into a slice
			commentTexts := make([]string, len(comments))
			for i, c := range comments {
				commentTexts[i] = c.Text
			}
			stats.Comments = commentTexts
		}

		// Update the map entry
		beadStats[beadID] = stats
	}
}
