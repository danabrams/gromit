package retro

import (
	"fmt"
	"os"
	"strings"

	"github.com/danabrams/gromit/internal/learnings"
)

// ApplyProposals applies retro proposals to learnings and rules files.
func ApplyProposals(proposals *Proposals, lf *learnings.File, rulesPath string) error {
	if proposals == nil {
		return nil
	}
	if lf == nil {
		return fmt.Errorf("learnings file is nil")
	}

	for _, consolidation := range proposals.Consolidations {
		if err := lf.Replace(consolidation.LearningHashes, consolidation.ConsolidatedText, learnings.CategoryPatterns); err != nil {
			return err
		}
	}
	for _, archive := range proposals.Archives {
		if err := lf.Archive(archive.LearningHash, archive.Rationale); err != nil {
			return err
		}
	}
	if len(proposals.Promotions) > 0 || len(proposals.RuleChanges) > 0 {
		if rulesPath == "" {
			return fmt.Errorf("rules path is empty")
		}
	}

	var rulesContent string
	rulesLoaded := false
	rulesDirty := false

	loadRules := func() error {
		if rulesLoaded {
			return nil
		}
		data, err := os.ReadFile(rulesPath)
		if err != nil {
			return fmt.Errorf("reading rules: %w", err)
		}
		rulesContent = string(data)
		rulesLoaded = true
		return nil
	}

	for _, promotion := range proposals.Promotions {
		if err := loadRules(); err != nil {
			return err
		}
		updated, err := insertRuleIntoSection(rulesContent, promotion.Section, promotion.ProposedRule)
		if err != nil {
			return err
		}
		rulesContent = updated
		rulesDirty = true
		if err := lf.Archive(promotion.LearningHash, promotion.Rationale); err != nil {
			return err
		}
	}

	for _, change := range proposals.RuleChanges {
		if err := loadRules(); err != nil {
			return err
		}
		if !strings.Contains(rulesContent, change.CurrentRule) {
			return fmt.Errorf("rule not found for change: %s", change.CurrentRule)
		}
		rulesContent = strings.Replace(rulesContent, change.CurrentRule, change.ProposedRule, 1)
		rulesDirty = true
	}

	if rulesDirty {
		if err := os.WriteFile(rulesPath, []byte(rulesContent), 0644); err != nil {
			return fmt.Errorf("writing rules: %w", err)
		}
	}

	return nil
}

func insertRuleIntoSection(content, section, rule string) (string, error) {
	if strings.TrimSpace(section) == "" {
		return "", fmt.Errorf("section is empty")
	}
	ruleLine := strings.TrimSpace(rule)
	if ruleLine == "" {
		return "", fmt.Errorf("rule is empty")
	}

	lines := strings.Split(content, "\n")
	sectionIndex := -1
	for i, line := range lines {
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		name := strings.TrimPrefix(line, "## ")
		if name == section || strings.HasPrefix(name, section+" ") {
			sectionIndex = i
			break
		}
	}
	if sectionIndex == -1 {
		return "", fmt.Errorf("section not found: %s", section)
	}

	nextHeader := len(lines)
	for i := sectionIndex + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			nextHeader = i
			break
		}
	}

	insertPos := nextHeader
	for insertPos > sectionIndex+1 && strings.TrimSpace(lines[insertPos-1]) == "" {
		insertPos--
	}

	lines = append(lines[:insertPos], append([]string{ruleLine}, lines[insertPos:]...)...)
	return strings.Join(lines, "\n"), nil
}
