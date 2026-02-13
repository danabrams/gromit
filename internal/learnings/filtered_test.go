package learnings

import (
	"strings"
	"testing"
	"time"
)

// setupConfirmedLearnings creates a File with N confirmed learnings of known sizes.
// Each learning has content "Learning-{i}: " followed by padding to reach targetCharLen.
// Learnings are ordered oldest-first (i=0 is oldest, i=n-1 is most recent).
func setupConfirmedLearnings(t *testing.T, n int, targetCharLen int) *File {
	t.Helper()
	f := &File{
		path:        t.TempDir() + "/LEARNINGS.md",
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}
	for i := 0; i < n; i++ {
		prefix := "Learning content for entry number "
		padding := strings.Repeat("x", targetCharLen-len(prefix)-5)
		content := prefix + padding
		f.confirmed = append(f.confirmed, Learning{
			Date:     time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			BeadID:   "bead-" + string(rune('a'+i)),
			Content:  content,
			Category: CategoryPatterns,
			Hash:     hashContent(content),
		})
	}
	return f
}

// TestGetConfirmedFiltered_CharBudgetPrefersMostRecent tests that when a character
// budget is applied, the most recently confirmed entries are preferred over older ones.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_CharBudgetPrefersMostRecent(t *testing.T) {
	// Create 5 learnings, each ~100 chars, with a budget of 250 chars.
	// Should return only the 2 most recent (indices 3 and 4), since the 3rd
	// would exceed the budget.
	f := setupConfirmedLearnings(t, 5, 100)

	opts := FilterOptions{MaxChars: 250}
	result := f.GetConfirmedFiltered(opts)

	if len(result) != 2 {
		t.Fatalf("expected 2 learnings within budget, got %d", len(result))
	}

	// The most recent entry (index 4, date Jan 5) should be included
	if result[0].Date != f.confirmed[4].Date {
		t.Errorf("expected most recent entry first, got date %v", result[0].Date)
	}
	// The second most recent (index 3, date Jan 4) should also be included
	if result[1].Date != f.confirmed[3].Date {
		t.Errorf("expected second most recent entry second, got date %v", result[1].Date)
	}
}

// TestGetConfirmedFiltered_ZeroMaxCharsMeansUnlimited tests that setting MaxChars
// to 0 returns all confirmed learnings without any truncation.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_ZeroMaxCharsMeansUnlimited(t *testing.T) {
	f := setupConfirmedLearnings(t, 5, 100)

	opts := FilterOptions{MaxChars: 0}
	result := f.GetConfirmedFiltered(opts)

	if len(result) != 5 {
		t.Errorf("expected all 5 learnings with zero MaxChars (unlimited), got %d", len(result))
	}
}

// TestGetConfirmedFiltered_KeywordFiltersCaseInsensitive tests that keyword filtering
// performs case-insensitive substring matching against learning Content.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_KeywordFiltersCaseInsensitive(t *testing.T) {
	f := &File{
		path:        t.TempDir() + "/LEARNINGS.md",
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}

	// Add learnings with distinct content
	entries := []struct {
		content  string
		category string
	}{
		{"Runner escalation chain skips haiku for high-complexity beads", CategoryPatterns},
		{"Config defaults must be set in SetDefaults() method", CategoryConventions},
		{"Validation retries use the same model tier", CategoryPatterns},
		{"Always run go test before committing changes", CategoryConventions},
	}
	for i, e := range entries {
		f.confirmed = append(f.confirmed, Learning{
			Date:     time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			BeadID:   "bead-" + string(rune('a'+i)),
			Content:  e.content,
			Category: e.category,
			Hash:     hashContent(e.content),
		})
	}

	tests := []struct {
		name     string
		keywords []string
		wantN    int
		wantAll  func([]Learning) bool
	}{
		{
			name:     "single keyword matches subset",
			keywords: []string{"escalation"},
			wantN:    1,
			wantAll: func(ls []Learning) bool {
				return strings.Contains(strings.ToLower(ls[0].Content), "escalation")
			},
		},
		{
			name:     "case insensitive match",
			keywords: []string{"SETDEFAULTS"},
			wantN:    1,
			wantAll: func(ls []Learning) bool {
				return strings.Contains(strings.ToLower(ls[0].Content), "setdefaults")
			},
		},
		{
			name:     "multiple keywords are OR-matched",
			keywords: []string{"escalation", "validation"},
			wantN:    2,
			wantAll: func(ls []Learning) bool {
				for _, l := range ls {
					lower := strings.ToLower(l.Content)
					if !strings.Contains(lower, "escalation") && !strings.Contains(lower, "validation") {
						return false
					}
				}
				return true
			},
		},
		{
			name:     "no keyword matches returns empty",
			keywords: []string{"nonexistent_keyword_xyz"},
			wantN:    0,
			wantAll:  func(ls []Learning) bool { return true },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := FilterOptions{Keywords: tt.keywords}
			result := f.GetConfirmedFiltered(opts)

			if len(result) != tt.wantN {
				t.Errorf("expected %d results, got %d", tt.wantN, len(result))
				for _, r := range result {
					t.Logf("  got: %s", r.Content[:50])
				}
				return
			}
			if !tt.wantAll(result) {
				t.Error("result entries did not match expected keyword filter")
			}
		})
	}
}

// TestGetConfirmedFiltered_KeywordsAndCharBudgetCombined tests that keyword filtering
// is applied first, then the character budget caps the result.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_KeywordsAndCharBudgetCombined(t *testing.T) {
	f := &File{
		path:        t.TempDir() + "/LEARNINGS.md",
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}

	// Create 4 learnings containing "runner", each ~100 chars
	for i := 0; i < 4; i++ {
		content := "Runner pattern " + strings.Repeat("x", 85)
		f.confirmed = append(f.confirmed, Learning{
			Date:     time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
			BeadID:   "bead-" + string(rune('a'+i)),
			Content:  content,
			Category: CategoryPatterns,
			Hash:     hashContent(content + string(rune('0'+i))), // unique hashes
		})
	}
	// Add 1 learning NOT containing "runner"
	f.confirmed = append(f.confirmed, Learning{
		Date:     time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
		BeadID:   "bead-z",
		Content:  "Config defaults must be set properly" + strings.Repeat("y", 65),
		Category: CategoryConventions,
		Hash:     hashContent("Config defaults must be set properly"),
	})

	// Filter by "runner" keyword, budget of 250 chars
	opts := FilterOptions{
		Keywords: []string{"runner"},
		MaxChars: 250,
	}
	result := f.GetConfirmedFiltered(opts)

	// Should only include "runner" entries, and further limited by budget
	for _, r := range result {
		if !strings.Contains(strings.ToLower(r.Content), "runner") {
			t.Errorf("expected all results to contain 'runner', got: %s", r.Content[:30])
		}
	}

	// With ~100 char entries and 250 budget, should get 2 entries
	if len(result) != 2 {
		t.Errorf("expected 2 entries after keyword+budget filter, got %d", len(result))
	}

	// The most recent matching entry should be first
	if len(result) >= 2 {
		if result[0].Date.Before(result[1].Date) {
			t.Error("expected most recent entries first when applying budget")
		}
	}
}

// TestGetConfirmedFiltered_EmptyConfirmedReturnsEmpty tests that filtering
// on an empty confirmed list returns an empty slice.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_EmptyConfirmedReturnsEmpty(t *testing.T) {
	f := &File{
		path:        t.TempDir() + "/LEARNINGS.md",
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}

	opts := FilterOptions{MaxChars: 1000, Keywords: []string{"anything"}}
	result := f.GetConfirmedFiltered(opts)

	if result == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results from empty confirmed, got %d", len(result))
	}
}

// TestGetConfirmedFiltered_NilReceiverReturnsEmpty tests nil-safe behavior.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_NilReceiverReturnsEmpty(t *testing.T) {
	var f *File

	opts := FilterOptions{MaxChars: 1000}
	result := f.GetConfirmedFiltered(opts)

	if result == nil {
		t.Error("expected non-nil empty slice from nil receiver, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results from nil receiver, got %d", len(result))
	}
}

// TestGetConfirmedFiltered_BudgetExactFit tests that an entry is included when it
// exactly fills the remaining budget.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_BudgetExactFit(t *testing.T) {
	f := &File{
		path:        t.TempDir() + "/LEARNINGS.md",
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}

	content := "exactly fifty characters long padding goes here!!!"
	if len(content) != 50 {
		t.Fatalf("test setup: content is %d chars, need 50", len(content))
	}

	f.confirmed = append(f.confirmed, Learning{
		Date:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BeadID:   "bead-a",
		Content:  content,
		Category: CategoryPatterns,
		Hash:     hashContent(content),
	})

	opts := FilterOptions{MaxChars: 50}
	result := f.GetConfirmedFiltered(opts)

	if len(result) != 1 {
		t.Errorf("expected 1 result when entry exactly fits budget, got %d", len(result))
	}
}

// TestGetConfirmedFiltered_SingleEntryExceedsBudget tests that when a single entry
// exceeds the budget, zero entries are returned.
// Expected failure: GetConfirmedFiltered method and FilterOptions struct do not exist yet.
func TestGetConfirmedFiltered_SingleEntryExceedsBudget(t *testing.T) {
	f := &File{
		path:        t.TempDir() + "/LEARNINGS.md",
		confirmed:   []Learning{},
		provisional: []Learning{},
		archived:    []Learning{},
	}

	content := strings.Repeat("x", 200)
	f.confirmed = append(f.confirmed, Learning{
		Date:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		BeadID:   "bead-a",
		Content:  content,
		Category: CategoryPatterns,
		Hash:     hashContent(content),
	})

	opts := FilterOptions{MaxChars: 50}
	result := f.GetConfirmedFiltered(opts)

	if len(result) != 0 {
		t.Errorf("expected 0 results when single entry exceeds budget, got %d", len(result))
	}
}
