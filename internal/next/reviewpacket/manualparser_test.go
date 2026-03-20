package reviewpacket

import (
	"strings"
	"testing"
)

func TestParseManualChecks_WithExplicitManualSection(t *testing.T) {
	content := `
# My Spec

## Validation

### Manual

#### Check: User can create a new item
Instructions: Navigate to the dashboard and click "Create Item"
Expected Result: A new item is created and appears in the list

#### Check: User can delete an item
Instructions: Select an item from the list and click "Delete"
Expected Result: The item is removed from the list

#### Check: Notification appears on success
Instructions: Observe the top of the page after deletion
Expected Result: A green notification message appears
`

	scenarios := []ParsedScenario{
		{Title: "User creates item", Given: "dashboard is open", When: "user clicks Create", Then: "item is created"},
		{Title: "User deletes item", Given: "item exists", When: "user clicks Delete", Then: "item is removed"},
	}

	checks := ParseManualChecks(content, scenarios)

	if len(checks.Items) != 3 {
		t.Fatalf("expected 3 manual checks, got %d", len(checks.Items))
	}

	// First check
	if checks.Items[0].Title != "User can create a new item" {
		t.Errorf("expected title 'User can create a new item', got '%s'", checks.Items[0].Title)
	}
	if checks.Items[0].Instructions != "Navigate to the dashboard and click \"Create Item\"" {
		t.Errorf("expected instructions, got '%s'", checks.Items[0].Instructions)
	}
	if checks.Items[0].ExpectedResult != "A new item is created and appears in the list" {
		t.Errorf("expected result, got '%s'", checks.Items[0].ExpectedResult)
	}
	if checks.Items[0].ID == "" {
		t.Errorf("expected ID to be set, got empty")
	}

	// Second check
	if checks.Items[1].Title != "User can delete an item" {
		t.Errorf("expected second title, got '%s'", checks.Items[1].Title)
	}

	// Third check
	if checks.Items[2].Title != "Notification appears on success" {
		t.Errorf("expected third title, got '%s'", checks.Items[2].Title)
	}
}

func TestParseManualChecks_NoManualSection_FallbackToScenarios(t *testing.T) {
	content := `
# My Spec

## Validation

Nothing here, no manual section.
`

	scenarios := []ParsedScenario{
		{Title: "User logs in successfully", Given: "user has valid credentials", When: "user submits login", Then: "user is logged in"},
		{Title: "User logs out", Given: "user is logged in", When: "user clicks logout", Then: "user is logged out"},
	}

	checks := ParseManualChecks(content, scenarios)

	if len(checks.Items) != 2 {
		t.Fatalf("expected 2 fallback checks from scenarios, got %d", len(checks.Items))
	}

	if checks.Items[0].Title != "User logs in successfully" {
		t.Errorf("expected first fallback title 'User logs in successfully', got '%s'", checks.Items[0].Title)
	}
	if checks.Items[1].Title != "User logs out" {
		t.Errorf("expected second fallback title 'User logs out', got '%s'", checks.Items[1].Title)
	}
}

func TestParseManualChecks_EmptyScenarios_EmptyResult(t *testing.T) {
	content := `
# My Spec

## Validation

No manual section and no scenarios to fall back to.
`

	scenarios := []ParsedScenario{}

	checks := ParseManualChecks(content, scenarios)

	if len(checks.Items) != 0 {
		t.Fatalf("expected 0 items with no scenarios, got %d", len(checks.Items))
	}
}

func TestParseManualChecks_GeneratesUniqueIDs(t *testing.T) {
	content := `
# Spec

## Validation

### Manual

#### Check: First check
Instructions: Do something
Expected Result: Get result

#### Check: Second check
Instructions: Do another thing
Expected Result: Get another result
`

	checks := ParseManualChecks(content, nil)

	if len(checks.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(checks.Items))
	}

	if checks.Items[0].ID == checks.Items[1].ID {
		t.Errorf("expected unique IDs, got same ID: %s", checks.Items[0].ID)
	}

	if checks.Items[0].ID == "" || checks.Items[1].ID == "" {
		t.Errorf("expected non-empty IDs")
	}
}

func TestParseManualChecks_WithBehaviorCardLinks(t *testing.T) {
	content := `
# Spec

## Validation

### Manual

#### Check: Verify login works
Instructions: Navigate to login page and enter credentials
Expected Result: Dashboard loads
Relates to: User logs in successfully, User with invalid creds

#### Check: Verify logout works
Instructions: Click logout button
Expected Result: Redirected to login page
Relates to: User logs out
`

	scenarios := []ParsedScenario{
		{Title: "User logs in successfully"},
		{Title: "User with invalid creds"},
		{Title: "User logs out"},
	}

	checks := ParseManualChecks(content, scenarios)

	if len(checks.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(checks.Items))
	}

	// First check should link to first and second scenarios
	if len(checks.Items[0].BehaviorCardIDs) != 2 {
		t.Errorf("expected 2 behavior card links in first check, got %d", len(checks.Items[0].BehaviorCardIDs))
	}

	// Second check should link to third scenario
	if len(checks.Items[1].BehaviorCardIDs) != 1 {
		t.Errorf("expected 1 behavior card link in second check, got %d", len(checks.Items[1].BehaviorCardIDs))
	}
}

func TestParseManualChecks_MultilineValues(t *testing.T) {
	content := `
# Spec

## Validation

### Manual

#### Check: Complex scenario
Instructions: This is a detailed instruction
  that spans multiple lines
  with important details
Expected Result: The system should show
  a success message
  and redirect to home
`

	checks := ParseManualChecks(content, nil)

	if len(checks.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(checks.Items))
	}

	// Should capture the key information from multiline values
	if !strings.Contains(checks.Items[0].Instructions, "detailed") {
		t.Errorf("expected instructions to contain 'detailed', got '%s'", checks.Items[0].Instructions)
	}
	if !strings.Contains(checks.Items[0].ExpectedResult, "success") {
		t.Errorf("expected result to contain 'success', got '%s'", checks.Items[0].ExpectedResult)
	}
}

func TestParseManualChecks_CaseInsensitiveKeywords(t *testing.T) {
	content := `
# Spec

## Validation

### Manual

#### Check: Test check
INSTRUCTIONS: Do the thing
EXPECTED RESULT: See the result
`

	checks := ParseManualChecks(content, nil)

	if len(checks.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(checks.Items))
	}

	if checks.Items[0].Instructions != "Do the thing" {
		t.Errorf("expected case-insensitive parsing of Instructions, got '%s'", checks.Items[0].Instructions)
	}
	if checks.Items[0].ExpectedResult != "See the result" {
		t.Errorf("expected case-insensitive parsing of ExpectedResult, got '%s'", checks.Items[0].ExpectedResult)
	}
}

func TestParseManualChecks_NoManualSection_EmptyContent(t *testing.T) {
	content := ""

	checks := ParseManualChecks(content, nil)

	if len(checks.Items) != 0 {
		t.Fatalf("expected 0 items for empty content, got %d", len(checks.Items))
	}
}

func TestParseManualChecks_PartialFields(t *testing.T) {
	content := `
# Spec

## Validation

### Manual

#### Check: Check with only instructions
Instructions: Do something important
`

	checks := ParseManualChecks(content, nil)

	if len(checks.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(checks.Items))
	}

	if checks.Items[0].Instructions != "Do something important" {
		t.Errorf("expected instructions, got '%s'", checks.Items[0].Instructions)
	}
	// ExpectedResult can be empty
	if checks.Items[0].ExpectedResult != "" {
		t.Errorf("expected empty ExpectedResult for partial check, got '%s'", checks.Items[0].ExpectedResult)
	}
}

func TestParseManualChecks_NumberedItems(t *testing.T) {
	content := `
# My Spec

## Validation

### Manual

1. Run a fixture spec that reaches ready_for_review and verify product-review.json exists
2. Verify product-review.md is readable and leads with behavior cards
3. Verify a blocked fixture run produces a diagnostic packet
`

	scenarios := []ParsedScenario{
		{Title: "Scenario 1", Given: "given", When: "when", Then: "then"},
		{Title: "Scenario 2", Given: "given", When: "when", Then: "then"},
	}

	checks := ParseManualChecks(content, scenarios)

	if len(checks.Items) != 3 {
		t.Fatalf("expected 3 explicit checks from numbered items, got %d", len(checks.Items))
	}

	// Check titles contain the numbered item text
	expectedTitles := []string{
		"Run a fixture spec that reaches ready_for_review and verify product-review.json exists",
		"Verify product-review.md is readable and leads with behavior cards",
		"Verify a blocked fixture run produces a diagnostic packet",
	}

	for i, expected := range expectedTitles {
		if checks.Items[i].Title != expected {
			t.Errorf("item %d: expected title '%s', got '%s'", i, expected, checks.Items[i].Title)
		}
		if checks.Items[i].ID == "" {
			t.Errorf("item %d: expected non-empty ID", i)
		}
	}
}

func TestParseManualChecks_NumberedItemsPreferredOverScenarios(t *testing.T) {
	// When explicit numbered items exist, they should be used instead of fallback to scenarios
	content := `
# My Spec

## Validation

### Manual

1. First manual step
2. Second manual step
3. Third manual step
`

	scenarios := []ParsedScenario{
		{Title: "Scenario 1", Given: "given", When: "when", Then: "then"},
		{Title: "Scenario 2", Given: "given", When: "when", Then: "then"},
	}

	checks := ParseManualChecks(content, scenarios)

	// Should use explicit 3 items, not fallback to 2 scenarios
	if len(checks.Items) != 3 {
		t.Fatalf("expected 3 explicit checks, got %d", len(checks.Items))
	}

	if checks.Items[0].Title != "First manual step" {
		t.Errorf("expected 'First manual step', got '%s'", checks.Items[0].Title)
	}
}

func TestParseManualChecks_MixedExplicitFormats(t *testing.T) {
	// Both "#### Check:" and numbered items should work together
	content := `
# My Spec

## Validation

### Manual

#### Check: Explicit check format
Instructions: Do this
Expected Result: See that

1. Numbered item format
`

	checks := ParseManualChecks(content, nil)

	if len(checks.Items) != 2 {
		t.Fatalf("expected 2 checks (one explicit header, one numbered), got %d", len(checks.Items))
	}

	if checks.Items[0].Title != "Explicit check format" {
		t.Errorf("expected first item to be explicit check, got '%s'", checks.Items[0].Title)
	}
	if checks.Items[1].Title != "Numbered item format" {
		t.Errorf("expected second item to be numbered, got '%s'", checks.Items[1].Title)
	}
}
