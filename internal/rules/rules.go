package rules

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Rules represents a parsed rules file with sections and rules
type Rules struct {
	Sections []Section
}

// Section represents a section in the rules file
type Section struct {
	Name  string
	Rules []string
}

// Load reads and parses a rules file from the given path
func Load(path string) (*Rules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading rules file: %w", err)
	}

	return Parse(string(data))
}

// Parse parses rules content from a string
func Parse(content string) (*Rules, error) {
	rules := &Rules{}
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentSection *Section

	for scanner.Scan() {
		line := scanner.Text()

		// Check for section header (## Section Name)
		if strings.HasPrefix(line, "## ") {
			sectionName := strings.TrimPrefix(line, "## ")
			currentSection = &Section{
				Name:  sectionName,
				Rules: []string{},
			}
			rules.Sections = append(rules.Sections, *currentSection)
			// Update pointer to the last section
			currentSection = &rules.Sections[len(rules.Sections)-1]
			continue
		}

		// Check for rule bullet (- Rule text)
		if strings.HasPrefix(line, "- ") && currentSection != nil {
			rule := strings.TrimPrefix(line, "- ")
			currentSection.Rules = append(currentSection.Rules, rule)
			continue
		}

		// Ignore other lines (comments, blank lines, etc.)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning rules file: %w", err)
	}

	return rules, nil
}

// Save writes the rules to a file at the given path
func (r *Rules) Save(path string) error {
	var builder strings.Builder

	// Write title/header if first section isn't named
	builder.WriteString("# Rules\n\n")
	builder.WriteString("These are non-negotiable constraints for ralph-runner development.\n\n")

	for i, section := range r.Sections {
		// Write section header
		builder.WriteString("## ")
		builder.WriteString(section.Name)
		builder.WriteString("\n\n")

		// Write rules
		for _, rule := range section.Rules {
			builder.WriteString("- ")
			builder.WriteString(rule)
			builder.WriteString("\n")
		}

		// Add blank line between sections (except after last)
		if i < len(r.Sections)-1 {
			builder.WriteString("\n")
		}
	}

	if err := os.WriteFile(path, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("writing rules file: %w", err)
	}

	return nil
}

// AddRule adds a new rule to the specified section
// If the section doesn't exist, it creates it
func (r *Rules) AddRule(sectionName string, rule string) {
	// Find existing section
	for i := range r.Sections {
		if r.Sections[i].Name == sectionName {
			r.Sections[i].Rules = append(r.Sections[i].Rules, rule)
			return
		}
	}

	// Section doesn't exist, create it
	r.Sections = append(r.Sections, Section{
		Name:  sectionName,
		Rules: []string{rule},
	})
}

// ModifyRule replaces an old rule with a new rule in the specified section
// Returns an error if the section or old rule is not found
func (r *Rules) ModifyRule(sectionName string, oldRule string, newRule string) error {
	// Find the section
	for i := range r.Sections {
		if r.Sections[i].Name == sectionName {
			// Find the old rule
			for j := range r.Sections[i].Rules {
				if r.Sections[i].Rules[j] == oldRule {
					r.Sections[i].Rules[j] = newRule
					return nil
				}
			}
			return fmt.Errorf("rule not found in section %q: %q", sectionName, oldRule)
		}
	}

	return fmt.Errorf("section not found: %q", sectionName)
}

// GetSection returns a section by name, or nil if not found
func (r *Rules) GetSection(name string) *Section {
	for i := range r.Sections {
		if r.Sections[i].Name == name {
			return &r.Sections[i]
		}
	}
	return nil
}

// RemoveRule removes a rule from the specified section
// Returns an error if the section or rule is not found
func (r *Rules) RemoveRule(sectionName string, rule string) error {
	// Find the section
	for i := range r.Sections {
		if r.Sections[i].Name == sectionName {
			// Find and remove the rule
			for j := range r.Sections[i].Rules {
				if r.Sections[i].Rules[j] == rule {
					r.Sections[i].Rules = append(
						r.Sections[i].Rules[:j],
						r.Sections[i].Rules[j+1:]...,
					)
					return nil
				}
			}
			return fmt.Errorf("rule not found in section %q: %q", sectionName, rule)
		}
	}

	return fmt.Errorf("section not found: %q", sectionName)
}
