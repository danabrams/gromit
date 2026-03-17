package contract

import (
	"fmt"
	"strings"
)

// ParseScenarios extracts scenarios from spec markdown by matching "### Scenario:" headers
// and parsing Given/When/Then/Notes blocks. Scenarios missing When or Then are skipped;
// their names with reasons are returned as the second return value. Given and Notes are
// optional. Returns empty slices (never nil) when no Scenarios section is found or it is empty.
func ParseScenarios(specMarkdown string) ([]SpecScenario, []string, error) {
	lines := strings.Split(specMarkdown, "\n")

	inScenariosSection := false
	var scenarios []SpecScenario
	var skipped []string

	// current scenario being built
	var cur *SpecScenario
	currentBlock := "" // which block we're currently collecting: "given","when","then","notes"
	var blockLines []string

	flushBlock := func() {
		if cur == nil || currentBlock == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(blockLines, "\n"))
		switch currentBlock {
		case "given":
			cur.Given = text
		case "when":
			cur.When = text
		case "then":
			cur.Then = text
		case "notes":
			cur.Notes = text
		}
		currentBlock = ""
		blockLines = nil
	}

	flushScenario := func() {
		if cur == nil {
			return
		}
		flushBlock()
		if cur.When == "" || cur.Then == "" {
			skipped = append(skipped, fmt.Sprintf("%s: missing When or Then block", cur.Name))
		} else {
			scenarios = append(scenarios, *cur)
		}
		cur = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect ## Scenarios section header (exact match)
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if heading == "Scenarios" {
				inScenariosSection = true
				continue
			}
			// Any other ## heading ends the Scenarios section
			if inScenariosSection {
				flushScenario()
				inScenariosSection = false
			}
			continue
		}

		if !inScenariosSection {
			continue
		}

		// Detect ### Scenario: header
		if strings.HasPrefix(trimmed, "### Scenario:") {
			flushScenario()
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "### Scenario:"))
			cur = &SpecScenario{Name: name}
			currentBlock = ""
			blockLines = nil
			continue
		}

		if cur == nil {
			continue
		}

		// Detect block markers: **Given:**, **When:**, **Then:**, **Notes:**
		if block, rest, ok := parseBlockMarker(trimmed); ok {
			flushBlock()
			currentBlock = block
			blockLines = nil
			if rest != "" {
				blockLines = append(blockLines, rest)
			}
			continue
		}

		// Accumulate into current block
		if currentBlock != "" {
			blockLines = append(blockLines, line)
		}
	}

	// Flush last scenario
	flushScenario()

	if scenarios == nil {
		scenarios = []SpecScenario{}
	}
	if skipped == nil {
		skipped = []string{}
	}
	return scenarios, skipped, nil
}

// parseBlockMarker checks if the line is a **Given:**, **When:**, **Then:**, or **Notes:** marker.
// Returns the block name (lowercased), the rest of the text on the same line, and true if matched.
func parseBlockMarker(line string) (block, rest string, ok bool) {
	for _, name := range []string{"Given", "When", "Then", "Notes"} {
		prefix := "**" + name + ":**"
		if strings.HasPrefix(line, prefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			return strings.ToLower(name), rest, true
		}
	}
	return "", "", false
}
