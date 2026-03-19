package reviewpacket

import (
	"fmt"
	"regexp"
	"strings"
)

// slugRe matches non-alphanumeric character runs for slug generation.
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// ParseManualChecks extracts manual verification checks from the spec content.
// It looks for an explicit "### Manual" section under "## Validation".
// If found, it parses individual check items.
// If not found, it falls back to deriving checks from scenarios.
// If no scenarios, it falls back to deriving from acceptance criteria.
func ParseManualChecks(content string, scenarios []ParsedScenario) ManualChecklist {
	lines := strings.Split(content, "\n")

	// Try to find and parse explicit manual checks from "## Validation > ### Manual"
	checks := parseExplicitManualChecks(lines)

	// If we found explicit checks, use them
	if len(checks) > 0 {
		return ManualChecklist{Items: checks}
	}

	// fallback: derive checks from scenarios if available
	if len(scenarios) > 0 {
		checks = deriveChecksFromScenarios(scenarios)
		return ManualChecklist{Items: checks}
	}

	// fallback: derive checks from acceptance criteria if no scenarios
	criteria := ParseAcceptanceCriteria(content)
	checks = deriveChecksFromCriteria(criteria)
	return ManualChecklist{Items: checks}
}

// parseExplicitManualChecks looks for the "## Validation" section,
// then "### Manual" subsection, and parses all check items.
func parseExplicitManualChecks(lines []string) []ManualCheckItem {
	var validationIdx int = -1
	var manualIdx int = -1

	// Find "## Validation" section
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "## Validation") {
			validationIdx = i
			break
		}
	}

	if validationIdx < 0 {
		return nil
	}

	// Find "### Manual" within Validation section (but before next ## section)
	for i := validationIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

		// Stop if we hit another ## section
		if strings.HasPrefix(trimmed, "##") && !strings.HasPrefix(trimmed, "###") {
			break
		}

		if strings.HasPrefix(trimmed, "### Manual") {
			manualIdx = i
			break
		}
	}

	if manualIdx < 0 {
		return nil
	}

	// Parse all checks starting from manualIdx
	var checks []ManualCheckItem
	i := manualIdx + 1

	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Look for check headers "#### Check: Title" first (before generic # check)
		if strings.HasPrefix(trimmed, "#### Check:") {
			check := parseManualCheckItem(lines, &i)
			checks = append(checks, check)
		} else if strings.HasPrefix(trimmed, "#") && len(trimmed) > 0 {
			// Stop if we hit another ## or ### section (but not ####)
			break
		} else {
			i++
		}
	}

	return checks
}

// parseManualCheckItem parses a single manual check starting at the current index.
// It extracts Title, Instructions, ExpectedResult, and Relates to field.
// It handles multiline values by collecting indented continuation lines.
func parseManualCheckItem(lines []string, idx *int) ManualCheckItem {
	line := strings.TrimSpace(lines[*idx])

	// Extract title from "#### Check: Title"
	title := strings.TrimPrefix(line, "#### Check:")
	title = strings.TrimSpace(title)

	checkID := generateCheckID(title)
	item := ManualCheckItem{
		ID:    checkID,
		Title: title,
	}

	*idx++

	// Parse instructions, expected result, and relates-to lines
	for *idx < len(lines) {
		line := lines[*idx]
		trimmed := strings.TrimSpace(line)

		// Stop if we hit another check or section header
		if strings.HasPrefix(trimmed, "####") || (strings.HasPrefix(trimmed, "#") && len(trimmed) > 0) {
			break
		}

		// Skip empty lines
		if trimmed == "" {
			*idx++
			continue
		}

		// Parse field lines (case-insensitive)
		lowerTrimmed := strings.ToLower(trimmed)

		if strings.HasPrefix(lowerTrimmed, "instructions:") {
			item.Instructions = extractFieldValueWithContinuation(lines, idx, "instructions")
			continue // extractFieldValueWithContinuation already incremented idx
		} else if strings.HasPrefix(lowerTrimmed, "expected result:") {
			item.ExpectedResult = extractFieldValueWithContinuation(lines, idx, "expected result")
			continue // extractFieldValueWithContinuation already incremented idx
		} else if strings.HasPrefix(lowerTrimmed, "relates to:") {
			item.BehaviorCardIDs = extractRelatesTo(trimmed)
		}

		*idx++
	}

	return item
}

// extractFieldValue extracts the value after a field keyword (case-insensitive).
func extractFieldValue(line, keyword string) string {
	lowerLine := strings.ToLower(line)
	keywordWithColon := keyword + ":"

	idx := strings.Index(lowerLine, keywordWithColon)
	if idx < 0 {
		return ""
	}

	// Extract content after keyword + colon + space
	content := line[idx+len(keywordWithColon):]
	content = strings.TrimSpace(content)

	return content
}

// extractFieldValueWithContinuation extracts a field value and handles multiline
// continuation. It looks ahead for indented lines and concatenates them.
// It increments the index pointer to move past all consumed lines.
func extractFieldValueWithContinuation(lines []string, idx *int, keyword string) string {
	line := lines[*idx]
	value := extractFieldValue(line, keyword)

	*idx++

	// Look for continuation lines (indented or blank lines before next field)
	var continuation []string
	for *idx < len(lines) {
		nextLine := lines[*idx]
		trimmed := strings.TrimSpace(nextLine)

		// Stop at headers or empty lines followed by a field keyword
		if strings.HasPrefix(trimmed, "#") {
			break
		}

		// Stop at field keywords (any case)
		lowerTrimmed := strings.ToLower(trimmed)
		if strings.HasPrefix(lowerTrimmed, "instructions:") ||
			strings.HasPrefix(lowerTrimmed, "expected result:") ||
			strings.HasPrefix(lowerTrimmed, "relates to:") {
			break
		}

		// Empty line: collect it but may signal end of field
		if trimmed == "" {
			*idx++
			// Check if next line is a field keyword or header
			if *idx < len(lines) {
				nextTrimmed := strings.TrimSpace(lines[*idx])
				nextLower := strings.ToLower(nextTrimmed)
				if strings.HasPrefix(nextTrimmed, "#") ||
					strings.HasPrefix(nextLower, "instructions:") ||
					strings.HasPrefix(nextLower, "expected result:") ||
					strings.HasPrefix(nextLower, "relates to:") {
					break
				}
			}
			continue
		}

		// Collect indented lines as continuation
		if len(nextLine) > 0 && (nextLine[0] == ' ' || nextLine[0] == '\t') {
			continuation = append(continuation, trimmed)
			*idx++
		} else {
			// Non-indented, non-empty line that's not a field keyword - stop
			break
		}
	}

	// Concatenate with continuation lines if any
	if len(continuation) > 0 {
		value = value + "\n" + strings.Join(continuation, "\n")
	}

	return value
}

// extractRelatesTo parses the "Relates to: scenario1, scenario2" field
// and returns a list of scenario title IDs that can be matched to behavior cards.
func extractRelatesTo(line string) []string {
	content := extractFieldValue(line, "relates to")
	if content == "" {
		return nil
	}

	// Split by comma and trim whitespace
	parts := strings.Split(content, ",")
	var ids []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			// For now, store the scenario title - caller will match to behavior card IDs
			ids = append(ids, trimmed)
		}
	}

	return ids
}

// deriveChecksFromScenarios creates manual check items from scenarios.
// Each scenario becomes a check item with Given/When/Then as instructions.
func deriveChecksFromScenarios(scenarios []ParsedScenario) []ManualCheckItem {
	var checks []ManualCheckItem

	for _, scenario := range scenarios {
		check := ManualCheckItem{
			ID:    generateCheckID(scenario.Title),
			Title: scenario.Title,
			Instructions: fmt.Sprintf("Given: %s\nWhen: %s\nThen: %s",
				scenario.Given, scenario.When, scenario.Then),
			ExpectedResult: scenario.Then,
		}
		checks = append(checks, check)
	}

	return checks
}

// deriveChecksFromCriteria creates manual check items from acceptance criteria.
// Each criterion becomes a check item with the criterion text as instructions.
func deriveChecksFromCriteria(criteria []ParsedCriterion) []ManualCheckItem {
	var checks []ManualCheckItem

	for _, criterion := range criteria {
		check := ManualCheckItem{
			ID:             generateCheckID(criterion.Text),
			Title:          criterion.Text,
			Instructions:   fmt.Sprintf("Verify: %s", criterion.Text),
			ExpectedResult: criterion.Text,
		}
		checks = append(checks, check)
	}

	return checks
}

// generateCheckID creates a unique ID for a check based on its title.
// Uses a deterministic slug-like format.
func generateCheckID(title string) string {
	// Convert to lowercase, replace spaces with hyphens, keep only alphanumeric and hyphens
	slug := strings.ToLower(title)
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Add "check_" prefix
	if slug == "" {
		slug = "unnamed"
	}

	return "check_" + slug
}
