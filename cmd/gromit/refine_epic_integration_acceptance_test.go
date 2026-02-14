//go:build acceptance

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
)

// setupRefineTestEnvironment creates a test environment with epics, specs, and backlog
func setupRefineTestEnvironment(t *testing.T) (string, string, string, string) {
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	epicsDir := filepath.Join(gromitDir, "epics")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(epicsDir, 0755); err != nil {
		t.Fatalf("Failed to create epics dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("Failed to create specs dir: %v", err)
	}

	return tmpDir, gromitDir, epicsDir, specsDir
}

// TestRefinePromptWithEpicContext_IncludesEpicDocument tests that when backlog item references an epic, prompt includes epic document
func TestRefinePromptWithEpicContext_IncludesEpicDocument(t *testing.T) {
	// Expected failure: buildRefinePrompt does not yet call detectEpicFromContext or buildEpicContextSection
	// Expected behavior: buildRefinePrompt should detect epic from context and include epic document
	//
	// This test verifies the first acceptance criterion: when a backlog item's context field
	// contains a known epic ID, the refine prompt includes the full epic document.

	tmpDir, gromitDir, epicsDir, specsDir := setupRefineTestEnvironment(t)

	// Create epic file
	epicContent := `---
epic_id: payment-integration
created: 2026-02-11
---

# Payment Integration Epic

This epic covers all payment processing features across the application.

## Vision

Enable users to pay with credit cards, PayPal, and cryptocurrency.

## Architecture

Payment gateway integration using Stripe API.
`
	epicPath := filepath.Join(epicsDir, "payment.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Create backlog file
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("Failed to create backlog file: %v", err)
	}

	// Add idea with epic context
	idea := &backlog.Idea{
		ID:      "idea-test-1",
		Text:    "Add payment confirmation page",
		Type:    "feature",
		Context: "Part of payment-integration epic",
		Status:  "pending",
	}
	if err := bf.Add(idea); err != nil {
		t.Fatalf("Failed to add idea to backlog: %v", err)
	}

	// Create pipeline and build prompt
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Specs:     specsDir,
		},
	}

	// Create mock dependencies
	mockAgentResolver := &mockAgentResolverForRefine{}
	mockBacklogClient := &mockBacklogClientForRefine{file: bf}

	deps := &pipeline.Deps{
		AgentResolver: mockAgentResolver,
		BacklogClient: mockBacklogClient,
	}

	paths := &pipeline.Paths{
		GromitDir: gromitDir,
		SpecsDir:  specsDir,
	}

	p := pipeline.New(deps, paths)

	// Load the idea and build prompt (simulating what runRefine does)
	loadedIdea, err := bf.Get("idea-test-1")
	if err != nil {
		t.Fatalf("Failed to load idea: %v", err)
	}

	ideaText := loadedIdea.Text
	if loadedIdea.Context != "" {
		ideaText = ideaText + "\n\nContext: " + loadedIdea.Context
	}

	// Call the buildRefinePrompt method (via reflection or indirect means)
	// Since buildRefinePrompt is private, we need to test through the public API
	// For now, we test that the integration works end-to-end by verifying
	// the system prompt includes epic context

	// NOTE: Since buildRefinePrompt is private in pipeline package,
	// we need to call it indirectly. The actual implementation should
	// expose this or we test via the full Refine() flow.
	// For this acceptance test, we're testing the behavior that should exist.

	// Build prompt manually to verify expected behavior
	prompt := buildRefinePromptWithEpicContext(cfg, gromitDir, ideaText, specsDir)

	// Verify epic context section is present
	if !strings.Contains(prompt, "## Epic Context") {
		t.Error("Prompt should contain '## Epic Context' section header")
	}

	// Verify epic document content is included
	if !strings.Contains(prompt, "# Payment Integration Epic") {
		t.Error("Prompt should contain epic title from document")
	}
	if !strings.Contains(prompt, "Enable users to pay with credit cards, PayPal, and cryptocurrency") {
		t.Error("Prompt should contain epic vision from document")
	}
	if !strings.Contains(prompt, "Payment gateway integration using Stripe API") {
		t.Error("Prompt should contain epic architecture from document")
	}

	// Verify frontmatter instruction
	if !strings.Contains(prompt, "Include `epic: payment-integration` in the spec frontmatter") {
		t.Error("Prompt should instruct Claude to add epic field to frontmatter")
	}

	// Verify epic is positioned between idea text and specs directory
	ideaIndex := strings.Index(prompt, "Add payment confirmation page")
	epicIndex := strings.Index(prompt, "## Epic Context")
	specsIndex := strings.Index(prompt, "Specs directory:")

	if ideaIndex == -1 || epicIndex == -1 || specsIndex == -1 {
		t.Fatal("Prompt missing required sections")
	}

	if !(ideaIndex < epicIndex && epicIndex < specsIndex) {
		t.Error("Epic Context section should appear between idea text and specs directory line")
	}
}

// TestRefinePromptWithEpicContext_IncludesSiblingSummaries tests that prompt includes sibling spec summaries
func TestRefinePromptWithEpicContext_IncludesSiblingSummaries(t *testing.T) {
	// Expected failure: buildRefinePrompt does not yet include sibling spec summaries
	// Expected behavior: buildRefinePrompt should include sibling spec titles and acceptance criteria
	//
	// This test verifies the third acceptance criterion: sibling spec summaries include
	// only the title and acceptance criteria, not full spec content.

	tmpDir, gromitDir, epicsDir, specsDir := setupRefineTestEnvironment(t)

	// Create epic file
	epicContent := `---
epic_id: auth-system
created: 2026-02-11
---

# Authentication System Epic

Complete authentication infrastructure.
`
	epicPath := filepath.Join(epicsDir, "auth.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Create sibling spec 1
	spec1Content := `---
id: login-api
epic: auth-system
created: 2026-02-11
---

# Login API

## Specification

Full detailed specification with lots of content that should NOT appear in summary.

## Acceptance Criteria

- API accepts username and password
- API returns JWT token on success
- API returns 401 on invalid credentials

## Research & Context

Detailed research notes that should not appear.
`
	spec1Path := filepath.Join(specsDir, "login-api.md")
	if err := os.WriteFile(spec1Path, []byte(spec1Content), 0644); err != nil {
		t.Fatalf("Failed to write spec1: %v", err)
	}

	// Create sibling spec 2
	spec2Content := `---
id: oauth-provider
epic: auth-system
created: 2026-02-11
---

# OAuth Provider Integration

## Specification

OAuth implementation details.

## Acceptance Criteria

- System supports Google OAuth
- System supports GitHub OAuth

## Decisions

Decision content that should not appear.
`
	spec2Path := filepath.Join(specsDir, "oauth-provider.md")
	if err := os.WriteFile(spec2Path, []byte(spec2Content), 0644); err != nil {
		t.Fatalf("Failed to write spec2: %v", err)
	}

	// Create backlog file and idea
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("Failed to create backlog file: %v", err)
	}

	idea := &backlog.Idea{
		ID:      "idea-test-2",
		Text:    "Add password reset flow",
		Type:    "feature",
		Context: "auth-system",
		Status:  "pending",
	}
	if err := bf.Add(idea); err != nil {
		t.Fatalf("Failed to add idea: %v", err)
	}

	// Build prompt
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Specs:     specsDir,
		},
	}

	loadedIdea, _ := bf.Get("idea-test-2")
	ideaText := loadedIdea.Text + "\n\nContext: " + loadedIdea.Context

	prompt := buildRefinePromptWithEpicContext(cfg, gromitDir, ideaText, specsDir)

	// Verify sibling specs section exists
	if !strings.Contains(prompt, "### Sibling Specs") {
		t.Error("Prompt should contain '### Sibling Specs' section")
	}

	// Verify spec titles are present
	if !strings.Contains(prompt, "**Login API**") {
		t.Error("Prompt should contain first sibling spec title")
	}
	if !strings.Contains(prompt, "**OAuth Provider Integration**") {
		t.Error("Prompt should contain second sibling spec title")
	}

	// Verify spec IDs are present
	if !strings.Contains(prompt, "(`login-api`)") {
		t.Error("Prompt should contain first sibling spec ID")
	}
	if !strings.Contains(prompt, "(`oauth-provider`)") {
		t.Error("Prompt should contain second sibling spec ID")
	}

	// Verify acceptance criteria are included
	acceptanceCriteria := []string{
		"API accepts username and password",
		"API returns JWT token on success",
		"API returns 401 on invalid credentials",
		"System supports Google OAuth",
		"System supports GitHub OAuth",
	}

	for _, criterion := range acceptanceCriteria {
		if !strings.Contains(prompt, criterion) {
			t.Errorf("Prompt should contain acceptance criterion: %q", criterion)
		}
	}

	// Verify full spec content is NOT included
	exclusions := []string{
		"Full detailed specification with lots of content that should NOT appear",
		"Detailed research notes that should not appear",
		"OAuth implementation details",
		"Decision content that should not appear",
	}

	for _, excluded := range exclusions {
		if strings.Contains(prompt, excluded) {
			t.Errorf("Prompt should NOT contain full spec content: %q", excluded)
		}
	}
}

// TestRefinePromptWithoutEpicContext_UnchangedBehavior tests that prompt without epic is unchanged
func TestRefinePromptWithoutEpicContext_UnchangedBehavior(t *testing.T) {
	// Expected failure: current implementation doesn't distinguish epic vs non-epic prompts
	// Expected behavior: when no epic is detected, prompt should match current behavior
	//
	// This test verifies the second acceptance criterion: when no epic ID matches,
	// the refine prompt is unchanged from current behavior.

	tmpDir, gromitDir, epicsDir, specsDir := setupRefineTestEnvironment(t)

	// Create an epic file (but don't reference it)
	epicContent := `---
epic_id: unrelated-epic
created: 2026-02-11
---

# Unrelated Epic
`
	epicPath := filepath.Join(epicsDir, "unrelated.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Create backlog file and idea WITHOUT epic context
	bf, err := backlog.NewFile(gromitDir)
	if err != nil {
		t.Fatalf("Failed to create backlog file: %v", err)
	}

	idea := &backlog.Idea{
		ID:      "idea-test-3",
		Text:    "Add user profile editing",
		Type:    "feature",
		Context: "Standalone feature not part of any epic",
		Status:  "pending",
	}
	if err := bf.Add(idea); err != nil {
		t.Fatalf("Failed to add idea: %v", err)
	}

	// Build prompt
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Specs:     specsDir,
		},
	}

	loadedIdea, _ := bf.Get("idea-test-3")
	ideaText := loadedIdea.Text + "\n\nContext: " + loadedIdea.Context

	prompt := buildRefinePromptWithEpicContext(cfg, gromitDir, ideaText, specsDir)

	// Verify NO epic context section
	if strings.Contains(prompt, "## Epic Context") {
		t.Error("Prompt should NOT contain '## Epic Context' section when no epic is detected")
	}

	// Verify NO epic document content
	if strings.Contains(prompt, "### Epic Document") {
		t.Error("Prompt should NOT contain '### Epic Document' section when no epic is detected")
	}

	// Verify NO sibling specs section
	if strings.Contains(prompt, "### Sibling Specs") {
		t.Error("Prompt should NOT contain '### Sibling Specs' section when no epic is detected")
	}

	// Verify standard prompt structure remains
	if !strings.Contains(prompt, "Add user profile editing") {
		t.Error("Prompt should still contain the idea text")
	}
	if !strings.Contains(prompt, "Specs directory:") {
		t.Error("Prompt should still contain specs directory line")
	}
}

// TestRefinePromptWithEpicContext_BlankSessionNoEpic tests that blank sessions don't attempt epic detection
func TestRefinePromptWithEpicContext_BlankSessionNoEpic(t *testing.T) {
	// Expected failure: blank session handling needs to skip epic detection
	// Expected behavior: blank sessions (no idea text or ID) skip epic detection
	//
	// This test verifies that ad-hoc ideas and blank sessions skip epic detection
	// as mentioned in the spec description.

	tmpDir, gromitDir, epicsDir, specsDir := setupRefineTestEnvironment(t)

	// Create an epic file
	epicContent := `---
epic_id: test-epic
created: 2026-02-11
---

# Test Epic
`
	epicPath := filepath.Join(epicsDir, "test.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Build prompt for blank session (empty idea text)
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Specs:     specsDir,
		},
	}

	prompt := buildRefinePromptWithEpicContext(cfg, gromitDir, "", specsDir)

	// Verify NO epic context
	if strings.Contains(prompt, "## Epic Context") {
		t.Error("Blank session prompt should NOT contain epic context")
	}

	// Verify standard blank session structure
	if !strings.Contains(prompt, "Specs directory:") {
		t.Error("Blank session prompt should contain specs directory")
	}
}

// TestRefinePromptWithEpicContext_AdHocIdeaNoEpic tests that ad-hoc ideas don't trigger epic detection
func TestRefinePromptWithEpicContext_AdHocIdeaNoEpic(t *testing.T) {
	// Expected failure: ad-hoc idea handling needs to skip epic detection
	// Expected behavior: ad-hoc ideas (direct text input) skip epic detection
	//
	// This test verifies that when users provide ad-hoc idea text (not from backlog),
	// no epic detection occurs even if the text contains epic ID strings.

	tmpDir, gromitDir, epicsDir, specsDir := setupRefineTestEnvironment(t)

	// Create an epic file
	epicContent := `---
epic_id: payment-integration
created: 2026-02-11
---

# Payment Integration Epic
`
	epicPath := filepath.Join(epicsDir, "payment.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Build prompt with ad-hoc text that happens to mention epic ID
	cfg := &config.Config{
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Specs:     specsDir,
		},
	}

	adHocText := "Implement payment-integration feature for checkout"

	prompt := buildRefinePromptWithEpicContext(cfg, gromitDir, adHocText, specsDir)

	// Verify NO epic context (because it's ad-hoc, not from backlog)
	if strings.Contains(prompt, "## Epic Context") {
		t.Error("Ad-hoc idea prompt should NOT contain epic context even if text mentions epic ID")
	}

	// Verify idea text is present
	if !strings.Contains(prompt, adHocText) {
		t.Error("Prompt should contain the ad-hoc idea text")
	}
}

// TestRefinePromptWithEpicContext_SubstringMatching tests that detection works with various substring formats
func TestRefinePromptWithEpicContext_SubstringMatching(t *testing.T) {
	// Expected failure: detectEpicFromContext doesn't exist yet in refine flow
	// Expected behavior: epic detection handles various substring formats
	//
	// This test verifies the fifth acceptance criterion: detection handles substring matching
	// for formats like "Part of X epic", "X", truncated strings containing X.

	tmpDir, gromitDir, epicsDir, specsDir := setupRefineTestEnvironment(t)

	// Create epic file
	epicContent := `---
epic_id: authentication-refactor
created: 2026-02-11
---

# Authentication Refactor Epic

Modernizing authentication.
`
	epicPath := filepath.Join(epicsDir, "auth-refactor.md")
	if err := os.WriteFile(epicPath, []byte(epicContent), 0644); err != nil {
		t.Fatalf("Failed to write epic file: %v", err)
	}

	// Test various context formats
	testCases := []struct {
		name    string
		context string
	}{
		{"prefix format", "Part of authentication-refactor epic"},
		{"bare ID", "authentication-refactor"},
		{"truncated", "f authentication-refac"},
		{"embedded", "This relates to the authentication-refactor effort"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create backlog file
			bf, err := backlog.NewFile(gromitDir)
			if err != nil {
				t.Fatalf("Failed to create backlog file: %v", err)
			}

			idea := &backlog.Idea{
				ID:      "idea-substring-" + tc.name,
				Text:    "Test idea",
				Type:    "feature",
				Context: tc.context,
				Status:  "pending",
			}
			if err := bf.Add(idea); err != nil {
				t.Fatalf("Failed to add idea: %v", err)
			}

			// Build prompt
			cfg := &config.Config{
				Paths: config.PathsConfig{
					GromitDir: gromitDir,
					Specs:     specsDir,
				},
			}

			loadedIdea, _ := bf.Get(idea.ID)
			ideaText := loadedIdea.Text + "\n\nContext: " + loadedIdea.Context

			prompt := buildRefinePromptWithEpicContext(cfg, gromitDir, ideaText, specsDir)

			// Verify epic was detected
			if !strings.Contains(prompt, "## Epic Context") {
				t.Errorf("Context format %q should trigger epic detection", tc.context)
			}
			if !strings.Contains(prompt, "# Authentication Refactor Epic") {
				t.Errorf("Epic document should be included for context %q", tc.context)
			}
		})
	}
}

// Mock types for testing

type mockAgentResolverForRefine struct{}

func (m *mockAgentResolverForRefine) Resolve(phase string, flagOverride string, choosePicker bool) (pipeline.Agent, error) {
	return nil, nil
}

type mockBacklogClientForRefine struct {
	file *backlog.File
}

func (m *mockBacklogClientForRefine) List() ([]*pipeline.Idea, error) {
	return nil, nil
}

func (m *mockBacklogClientForRefine) Get(id string) (*pipeline.Idea, error) {
	idea, err := m.file.Get(id)
	if err != nil || idea == nil {
		return nil, err
	}
	return &pipeline.Idea{
		ID:      idea.ID,
		Text:    idea.Text,
		Type:    idea.Type,
		Context: idea.Context,
		Status:  idea.Status,
	}, nil
}

func (m *mockBacklogClientForRefine) Add(item *pipeline.Idea) error {
	return nil
}

func (m *mockBacklogClientForRefine) Update(id string, fn func(*pipeline.Idea)) error {
	return nil
}

// buildRefinePromptWithEpicContext is the function that WILL exist after implementation
// Expected signature: func buildRefinePromptWithEpicContext(cfg *config.Config, gromitDir string, ideaText string, specsDir string) string
//
// This function should:
// 1. Check if ideaText contains a backlog context field
// 2. Call detectEpicFromContext with the context
// 3. If epic found, call buildEpicContextSection
// 4. Insert epic section between idea text and specs directory
// 5. Return the complete prompt
//
// For now, this is a stub that will fail when called.
func buildRefinePromptWithEpicContext(cfg *config.Config, gromitDir string, ideaText string, specsDir string) string {
	// This function does not exist yet - tests will fail here
	panic("buildRefinePromptWithEpicContext not implemented yet")
}
