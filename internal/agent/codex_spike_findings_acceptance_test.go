//go:build acceptance

package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodexSpikeFindingsDocumentExists verifies that the spike findings document
// has been created and contains the required sections for all seven test areas.
//
// Expected failure: The findings document does not exist yet because the manual
// spike investigation has not been completed.
func TestCodexSpikeFindingsDocumentExists(t *testing.T) {
	// The findings should be documented in .gromit/spikes/codex-cli-findings.md
	findingsPath := filepath.Join(".gromit", "spikes", "codex-cli-findings.md")

	// Verify the findings file exists
	if _, err := os.Stat(findingsPath); os.IsNotExist(err) {
		t.Errorf("Codex CLI spike findings document not found at %s", findingsPath)
		return
	}

	// Read the findings document
	content, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Fatalf("Failed to read findings document: %v", err)
	}

	findingsContent := string(content)

	// Verify all seven required sections are present
	requiredSections := []struct {
		name    string
		markers []string // Any of these markers should be present
	}{
		{
			name:    "Authentication",
			markers: []string{"## 1. Authentication", "## Authentication", "### Authentication"},
		},
		{
			name:    "Prompt File Delivery",
			markers: []string{"## 2. Prompt File Delivery", "## Prompt File Delivery", "## Prompt Delivery"},
		},
		{
			name:    "Model Flag Format",
			markers: []string{"## 3. Model Flag Format", "## Model Flag", "## Model Selection"},
		},
		{
			name:    "Exit Codes",
			markers: []string{"## 4. Exit Codes", "## Exit Codes"},
		},
		{
			name:    "Output Format",
			markers: []string{"## 5. Output Format", "## Output Format"},
		},
		{
			name:    "Approval Mode Flags",
			markers: []string{"## 6. Approval Mode", "## Approval Mode Flags", "## Approval Flags"},
		},
		{
			name:    "Working Directory",
			markers: []string{"## 7. Working Directory", "## Working Directory", "## File Access"},
		},
	}

	var missingSections []string
	for _, section := range requiredSections {
		found := false
		for _, marker := range section.markers {
			if strings.Contains(findingsContent, marker) {
				found = true
				break
			}
		}
		if !found {
			missingSections = append(missingSections, section.name)
		}
	}

	if len(missingSections) > 0 {
		t.Errorf("Findings document missing required sections: %v", missingSections)
	}
}

// TestCodexSpikeFindingsAnswersKeyQuestions verifies that the findings document
// answers the seven critical questions needed for CodexProvider implementation.
//
// Expected failure: The findings document does not contain answers to these
// questions because the manual investigation has not been completed yet.
func TestCodexSpikeFindingsAnswersKeyQuestions(t *testing.T) {
	findingsPath := filepath.Join(".gromit", "spikes", "codex-cli-findings.md")

	content, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Skipf("Findings document not found, skipping question verification: %v", err)
		return
	}

	findingsContent := strings.ToLower(string(content))

	// Each question requires certain keywords/concepts to be addressed
	keyQuestions := []struct {
		question string
		keywords []string // At least one of these should be present
	}{
		{
			question: "Prompt delivery method",
			keywords: []string{"--prompt", "prompt file", "stdin", "positional"},
		},
		{
			question: "Model selection syntax",
			keywords: []string{"--model", "model flag", "o3", "gpt-4"},
		},
		{
			question: "Exit codes for usage limits",
			keywords: []string{"exit code", "usage limit", "rate limit", "error code"},
		},
		{
			question: "Output capture method",
			keywords: []string{"stdout", "stderr", "output", "stream", "json"},
		},
		{
			question: "Automation/approval flags",
			keywords: []string{"approval", "auto", "full-auto", "suggest", "autonomous"},
		},
		{
			question: "Working directory behavior",
			keywords: []string{"working directory", "cwd", "project-dir", "directory"},
		},
		{
			question: "Blockers or showstoppers",
			keywords: []string{"blocker", "showstopper", "limitation", "issue", "problem", "none", "no blocker"},
		},
	}

	var unansweredQuestions []string
	for _, q := range keyQuestions {
		found := false
		for _, keyword := range q.keywords {
			if strings.Contains(findingsContent, strings.ToLower(keyword)) {
				found = true
				break
			}
		}
		if !found {
			unansweredQuestions = append(unansweredQuestions, q.question)
		}
	}

	if len(unansweredQuestions) > 0 {
		t.Errorf("Findings document does not address these key questions: %v", unansweredQuestions)
	}
}

// TestCodexSpikeFindingsAreDetailed verifies that the findings document
// contains sufficient detail rather than just placeholder text.
//
// Expected failure: The findings document either does not exist or contains
// only template/placeholder text because the investigation is not complete.
func TestCodexSpikeFindingsAreDetailed(t *testing.T) {
	findingsPath := filepath.Join(".gromit", "spikes", "codex-cli-findings.md")

	content, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Skipf("Findings document not found, skipping detail verification: %v", err)
		return
	}

	findingsContent := string(content)

	// Check for minimum content length (should be substantial, not just a template)
	if len(findingsContent) < 1000 {
		t.Errorf("Findings document is too short (%d bytes), expected detailed findings with examples and observations", len(findingsContent))
	}

	// Check for presence of actual command examples (bash code blocks)
	if !strings.Contains(findingsContent, "```") && !strings.Contains(findingsContent, "codex ") {
		t.Error("Findings document should contain actual command examples from the investigation")
	}

	// Check that it's not just a template (look for placeholder markers)
	placeholderMarkers := []string{
		"[TODO]",
		"[TBD]",
		"<fill in>",
		"xxx",
		"...",
	}

	for _, marker := range placeholderMarkers {
		if strings.Contains(strings.ToLower(findingsContent), strings.ToLower(marker)) {
			t.Errorf("Findings document contains placeholder text (%s), indicating incomplete investigation", marker)
		}
	}
}

// TestCodexSpikeFindingsIdentifyImplementationRequirements verifies that the
// findings document includes concrete implementation details for CodexProvider.
//
// Expected failure: The findings document does not include implementation
// requirements because the manual spike has not been completed yet.
func TestCodexSpikeFindingsIdentifyImplementationRequirements(t *testing.T) {
	findingsPath := filepath.Join(".gromit", "spikes", "codex-cli-findings.md")

	content, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Skipf("Findings document not found, skipping implementation requirements check: %v", err)
		return
	}

	findingsContent := strings.ToLower(string(content))

	// The findings should provide implementation guidance
	implementationConcepts := []string{
		"codexprovider", // Should mention the target implementation
		"provider",      // Should reference the Provider interface
		"implement",     // Should discuss what needs to be implemented
		"flag",          // Should specify exact flags to use
		"command",       // Should show command construction
	}

	foundConcepts := 0
	for _, concept := range implementationConcepts {
		if strings.Contains(findingsContent, concept) {
			foundConcepts++
		}
	}

	// Should mention at least 3 of these implementation-related concepts
	if foundConcepts < 3 {
		t.Errorf("Findings document lacks implementation guidance (found %d/%d key concepts)", foundConcepts, len(implementationConcepts))
	}
}

// TestCodexSpikeDocumentsBlockersOrWorkarounds verifies that if blockers are
// found, they are documented with proposed workarounds.
//
// Expected failure: The findings document does not exist yet, so blocker
// documentation cannot be verified.
func TestCodexSpikeDocumentsBlockersOrWorkarounds(t *testing.T) {
	findingsPath := filepath.Join(".gromit", "spikes", "codex-cli-findings.md")

	content, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Skipf("Findings document not found, skipping blocker verification: %v", err)
		return
	}

	findingsContent := strings.ToLower(string(content))

	// Check if blockers are mentioned
	hasBlockerSection := strings.Contains(findingsContent, "blocker") ||
		strings.Contains(findingsContent, "limitation") ||
		strings.Contains(findingsContent, "showstopper") ||
		strings.Contains(findingsContent, "issue")

	// If blockers are mentioned, there should also be workarounds or solutions
	if hasBlockerSection {
		hasWorkaround := strings.Contains(findingsContent, "workaround") ||
			strings.Contains(findingsContent, "alternative") ||
			strings.Contains(findingsContent, "solution") ||
			strings.Contains(findingsContent, "instead")

		if !hasWorkaround {
			t.Error("Findings document mentions blockers but does not provide workarounds or alternatives")
		}
	}

	// The findings should have a clear conclusion about feasibility
	hasFeasibility := strings.Contains(findingsContent, "feasible") ||
		strings.Contains(findingsContent, "possible") ||
		strings.Contains(findingsContent, "can be implemented") ||
		strings.Contains(findingsContent, "cannot be implemented")

	if !hasFeasibility {
		t.Error("Findings document should include a feasibility assessment for CodexProvider implementation")
	}
}

// TestCodexSpikeDocumentStructure verifies the overall structure and organization
// of the findings document.
//
// Expected failure: The findings document does not exist yet with the expected
// structure because the spike investigation has not been completed.
func TestCodexSpikeDocumentStructure(t *testing.T) {
	findingsPath := filepath.Join(".gromit", "spikes", "codex-cli-findings.md")

	content, err := os.ReadFile(findingsPath)
	if err != nil {
		t.Skipf("Findings document not found, skipping structure verification: %v", err)
		return
	}

	findingsContent := string(content)

	// Should have a clear title/heading
	if !strings.HasPrefix(strings.TrimSpace(findingsContent), "#") {
		t.Error("Findings document should start with a markdown heading")
	}

	// Should have markdown formatting
	headingCount := strings.Count(findingsContent, "\n## ")
	if headingCount < 7 {
		t.Errorf("Findings document should have at least 7 section headings (one per test area), found %d", headingCount)
	}

	// Should have a summary or conclusion section
	hasSummary := strings.Contains(strings.ToLower(findingsContent), "## summary") ||
		strings.Contains(strings.ToLower(findingsContent), "## conclusion") ||
		strings.Contains(strings.ToLower(findingsContent), "## findings") ||
		strings.Contains(strings.ToLower(findingsContent), "## next steps")

	if !hasSummary {
		t.Error("Findings document should have a summary or conclusion section")
	}
}
