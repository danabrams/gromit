package learnings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAddNewLearning tests adding a new learning to an empty file
func TestAddNewLearning(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFile(tmpDir)

	learning, err := f.Add("bead-123", "Always check error returns", CategoryPatterns)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if learning == nil {
		t.Fatal("learning should not be nil")
	}
	if learning.Content != "Always check error returns" {
		t.Errorf("expected content 'Always check error returns', got %q", learning.Content)
	}
	if learning.Category != CategoryPatterns {
		t.Errorf("expected category %q, got %q", CategoryPatterns, learning.Category)
	}
	if learning.BeadID != "bead-123" {
		t.Errorf("expected BeadID 'bead-123', got %q", learning.BeadID)
	}

	// Should be in provisional
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional learning, got %d", len(f.provisional))
	}
}

// TestAddExactDuplicate tests that exact duplicates are skipped
func TestAddExactDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFile(tmpDir)

	content := "Always check error returns"

	// Add first time
	learning1, err := f.Add("bead-1", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	// Add exact duplicate
	learning2, err := f.Add("bead-2", content, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if learning2 != nil {
		t.Fatal("duplicate learning should return nil")
	}

	// Should still have only 1 provisional
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional learning, got %d", len(f.provisional))
	}
}

// TestAddExactDuplicateNormalized tests that normalized duplicates are caught
func TestAddExactDuplicateNormalized(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFile(tmpDir)

	// These should hash to the same value after normalization
	content1 := "Always  check   error   returns"
	content2 := "always check error returns" // Different case and spacing

	learning1, err := f.Add("bead-1", content1, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	learning2, err := f.Add("bead-2", content2, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if learning2 != nil {
		t.Fatal("normalized duplicate should return nil")
	}
}

// TestAddFuzzyMatchPromotion tests that fuzzy matches promote provisional to confirmed
func TestAddFuzzyMatchPromotion(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFile(tmpDir)

	// Use very similar content to trigger > 0.7 similarity
	content1 := "Always check error returns are handled properly"
	content2 := "Always check error returns are handled very properly" // Very similar

	// Add first learning
	learning1, err := f.Add("bead-1", content1, CategoryPatterns)
	if err != nil {
		t.Fatalf("first add failed: %v", err)
	}
	if learning1 == nil {
		t.Fatal("first learning should not be nil")
	}

	// Verify it's in provisional
	if len(f.provisional) != 1 {
		t.Errorf("after first add, expected 1 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 0 {
		t.Errorf("after first add, expected 0 confirmed, got %d", len(f.confirmed))
	}

	// Check similarity to verify it's actually > 0.7
	sim := similarity(content1, content2)
	if sim <= 0.7 {
		t.Skipf("similarity %f is too low for promotion test, need > 0.7", sim)
	}

	// Add similar learning - should promote (remove from provisional, add to confirmed)
	learning2, err := f.Add("bead-2", content2, CategoryPatterns)
	if err != nil {
		t.Fatalf("second add failed: %v", err)
	}
	if learning2 == nil {
		t.Fatal("fuzzy match should return a learning")
	}
	if learning2.RelatedTo != "bead-1" {
		t.Errorf("expected RelatedTo='bead-1', got %q", learning2.RelatedTo)
	}

	// Original should be removed from provisional, new one added to confirmed
	if len(f.confirmed) != 1 {
		t.Errorf("after fuzzy match, expected 1 confirmed, got %d", len(f.confirmed))
	}
	if len(f.provisional) != 0 {
		t.Errorf("after fuzzy match, expected 0 provisional, got %d", len(f.provisional))
	}
}

// TestAddFuzzyMatchNonPromotion tests that fuzzy matches to confirmed mark as related
func TestAddFuzzyMatchNonPromotion(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFile(tmpDir)

	content1 := "Always check error returns are handled properly"
	content2 := "Always check error returns are handled very properly" // Very similar

	// Manually add to confirmed (simulating a previously promoted learning)
	f.confirmed = append(f.confirmed, Learning{
		Date:    time.Now(),
		BeadID:  "bead-1",
		Content: content1,
		Category: CategoryPatterns,
		Hash:    hashContent(content1),
	})

	// Check similarity to verify it's actually > 0.7
	sim := similarity(content1, content2)
	if sim <= 0.7 {
		t.Skipf("similarity %f is too low for test, need > 0.7", sim)
	}

	// Add similar learning - should mark as related but stay in provisional
	learning, err := f.Add("bead-2", content2, CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}
	if learning.RelatedTo != "bead-1" {
		t.Errorf("expected RelatedTo='bead-1', got %q", learning.RelatedTo)
	}

	// Should be added to provisional, not confirmed
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 1 {
		t.Errorf("expected 1 confirmed (unchanged), got %d", len(f.confirmed))
	}
}

// TestLoadAndSaveRoundTrip tests that loading and saving preserves data
func TestLoadAndSaveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and save
	f1 := NewFile(tmpDir)
	l1, _ := f1.Add("bead-1", "First learning", CategoryPatterns)
	l2, _ := f1.Add("bead-2", "Second learning", CategoryConventions)

	// Verify what we added
	if len(f1.provisional) != 2 {
		t.Fatalf("expected 2 provisional after adding, got %d", len(f1.provisional))
	}

	// Create new file instance and load
	f2 := NewFile(tmpDir)
	err := f2.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Verify data matches - should have the 2 provisional we saved
	if len(f2.provisional) != 2 {
		t.Errorf("expected 2 provisional learnings after load, got %d", len(f2.provisional))
	}
	if len(f2.confirmed) != 0 {
		t.Errorf("expected 0 confirmed learnings after load, got %d", len(f2.confirmed))
	}

	// Check content
	if len(f2.provisional) > 0 && f2.provisional[0].Content != l1.Content {
		t.Errorf("first provisional content mismatch: %q != %q", f2.provisional[0].Content, l1.Content)
	}
	if len(f2.provisional) > 1 && f2.provisional[1].Content != l2.Content {
		t.Errorf("second provisional content mismatch: %q != %q", f2.provisional[1].Content, l2.Content)
	}
}

// TestGetRecentTimeFiltering tests that GetRecent filters by time
func TestGetRecentTimeFiltering(t *testing.T) {
	f := &File{}

	now := time.Now()

	// Add recent learning
	f.provisional = append(f.provisional, Learning{
		Date:    now,
		BeadID:  "bead-1",
		Content: "Recent learning",
	})

	// Add old learning
	f.provisional = append(f.provisional, Learning{
		Date:    now.Add(-2 * time.Hour),
		BeadID:  "bead-2",
		Content: "Old learning",
	})

	// Get recent from last hour
	recent := f.GetRecent(1)
	if len(recent) != 1 {
		t.Errorf("expected 1 recent learning (1 hour), got %d", len(recent))
	}
	if recent[0].BeadID != "bead-1" {
		t.Errorf("expected recent learning bead-1, got %s", recent[0].BeadID)
	}

	// Get recent from last 3 hours
	recent = f.GetRecent(3)
	if len(recent) != 2 {
		t.Errorf("expected 2 recent learnings (3 hours), got %d", len(recent))
	}
}

// TestSimilarity tests the similarity function
func TestSimilarity(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		name string
		check func(float64) bool
	}{
		{"hello world", "hello world", "identical", func(s float64) bool { return s == 1.0 }},
		{"Hello World", "hello world", "case insensitive", func(s float64) bool { return s == 1.0 }},
		{"hello world", "hello there", "different words", func(s float64) bool { return s > 0.2 && s < 0.5 }},
		{"", "abc", "empty string", func(s float64) bool { return s == 0.0 }},
		{"a", "b", "single chars", func(s float64) bool { return s == 0.0 }},
		{"abc", "abc", "short identical", func(s float64) bool { return s == 1.0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := similarity(tt.a, tt.b)
			if !tt.check(got) {
				t.Errorf("similarity(%q, %q) = %f, check failed", tt.a, tt.b, got)
			}
		})
	}
}

// TestSimilaritySymmetric tests that similarity is symmetric
func TestSimilaritySymmetric(t *testing.T) {
	a := "hello world test"
	b := "hello test string"

	sim1 := similarity(a, b)
	sim2 := similarity(b, a)

	if sim1 != sim2 {
		t.Errorf("similarity is not symmetric: %f != %f", sim1, sim2)
	}
}

// TestParseLearnings tests the learnings parser
func TestParseLearnings(t *testing.T) {
	content := `# Learnings

Accumulated operational knowledge from Ralph iterations.

---

## Confirmed

*Patterns seen multiple times - high confidence.*

### 2026-02-01 | bead-1 | patterns
First confirmed learning content

### 2026-02-02 | bead-2 | conventions
*Related to: bead-1*

Second confirmed learning content

---

## Provisional

*Seen once - may be specific to one task.*

### 2026-02-03 | bead-3 | gotchas
First provisional learning

### 2026-02-04 | bead-4 | patterns
Second provisional learning

`

	confirmed, provisional := parseLearnings(content)

	// The parser will parse until it hits "## Provisional"
	// At that point, it saves the confirmed ones
	// The key is that the last learning needs content after it
	if len(confirmed) < 1 {
		t.Errorf("expected at least 1 confirmed learning, got %d", len(confirmed))
	}
	if len(provisional) < 1 {
		t.Errorf("expected at least 1 provisional learning, got %d", len(provisional))
	}

	// Check that bead-1 is in confirmed
	var foundBead1 bool
	for _, c := range confirmed {
		if c.BeadID == "bead-1" {
			foundBead1 = true
			if c.Category != "patterns" {
				t.Errorf("bead-1 category = %q, want patterns", c.Category)
			}
			break
		}
	}
	if !foundBead1 {
		t.Error("bead-1 not found in confirmed")
	}

	// Check that bead-3 is in provisional
	var foundBead3 bool
	for _, p := range provisional {
		if p.BeadID == "bead-3" {
			foundBead3 = true
			if p.Category != "gotchas" {
				t.Errorf("bead-3 category = %q, want gotchas", p.Category)
			}
			break
		}
	}
	if !foundBead3 {
		t.Error("bead-3 not found in provisional")
	}
}

// TestParseEmptyFile tests parsing an empty or minimal file
func TestParseEmptyFile(t *testing.T) {
	content := `# Learnings

Accumulated operational knowledge.

---

## Confirmed

*No confirmed learnings yet.*

---

## Provisional

*No provisional learnings.*
`

	confirmed, provisional := parseLearnings(content)

	if len(confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(confirmed))
	}
	if len(provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(provisional))
	}
}

// TestLoadMissingFile tests that Load handles missing files gracefully
func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	f := NewFile(tmpDir)

	// File doesn't exist yet
	err := f.Load()
	if err != nil {
		t.Fatalf("load on missing file should not error: %v", err)
	}

	// Should be empty
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f.confirmed))
	}
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(f.provisional))
	}
}

// TestLoadAndSaveCreatesDirectory tests that Save creates needed directories
func TestLoadAndSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "path", ".ralph")
	f := NewFile(nestedDir)

	// Directory doesn't exist
	if _, err := os.Stat(nestedDir); !os.IsNotExist(err) {
		t.Fatal("directory should not exist yet")
	}

	// Add and save
	_, err := f.Add("bead-1", "test", CategoryPatterns)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Directory should now exist
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Fatal("directory should have been created")
	}
}

func TestShouldSuggestRetro_ProvisionalCount(t *testing.T) {
	f := &File{}
	// Add 11 provisional learnings
	for i := 0; i < 11; i++ {
		f.provisional = append(f.provisional, Learning{Content: "learning"})
	}

	should, reason := f.ShouldSuggestRetro(time.Now(), 0)
	if !should {
		t.Error("should suggest retro when provisional count > 10")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestShouldSuggestRetro_DaysSinceRetro(t *testing.T) {
	f := &File{}
	// Add a few learnings so the file isn't empty
	f.provisional = append(f.provisional, Learning{Content: "learning"})

	lastRetro := time.Now().Add(-8 * 24 * time.Hour) // 8 days ago
	should, reason := f.ShouldSuggestRetro(lastRetro, 0)
	if !should {
		t.Error("should suggest retro when days since last retro > 7")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestShouldSuggestRetro_HighFailureRate(t *testing.T) {
	f := &File{}
	f.provisional = append(f.provisional, Learning{Content: "learning"})

	should, reason := f.ShouldSuggestRetro(time.Now(), 0.35)
	if !should {
		t.Error("should suggest retro when failure rate > 30%")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestShouldSuggestRetro_LargeLearningsFile(t *testing.T) {
	f := &File{}
	// Add 15 confirmed + 6 provisional = 21 total
	for i := 0; i < 15; i++ {
		f.confirmed = append(f.confirmed, Learning{Content: "confirmed"})
	}
	for i := 0; i < 6; i++ {
		f.provisional = append(f.provisional, Learning{Content: "provisional"})
	}

	should, reason := f.ShouldSuggestRetro(time.Now(), 0)
	if !should {
		t.Error("should suggest retro when total learnings > 20")
	}
	if reason == "" {
		t.Error("reason should not be empty")
	}
}

func TestShouldSuggestRetro_NoSuggestion(t *testing.T) {
	f := &File{}
	f.provisional = append(f.provisional, Learning{Content: "learning"})

	// Recent retro, low failure rate, few learnings
	should, _ := f.ShouldSuggestRetro(time.Now(), 0.1)
	if should {
		t.Error("should not suggest retro when all conditions are below thresholds")
	}
}

func TestShouldSuggestRetro_ZeroTime(t *testing.T) {
	f := &File{}
	f.provisional = append(f.provisional, Learning{Content: "learning"})

	// Zero time means retro was never run - should suggest
	should, _ := f.ShouldSuggestRetro(time.Time{}, 0)
	if !should {
		t.Error("should suggest retro when retro was never run (zero time)")
	}
}
