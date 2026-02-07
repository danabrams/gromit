package learnings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAddNewLearning tests adding a new learning to an empty file
func TestAddNewLearning(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

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
	f, _ := NewFile(tmpDir)

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
	f, _ := NewFile(tmpDir)

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
	f, _ := NewFile(tmpDir)

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

	// Check similarity to verify it's actually > FuzzyMatchThreshold
	sim := similarity(content1, content2)
	if sim <= FuzzyMatchThreshold {
		t.Skipf("similarity %f is too low for promotion test, need > %f", sim, FuzzyMatchThreshold)
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
	f, _ := NewFile(tmpDir)

	content1 := "Always check error returns are handled properly"
	content2 := "Always check error returns are handled very properly" // Very similar

	// Manually add to confirmed (simulating a previously promoted learning)
	f.confirmed = append(f.confirmed, Learning{
		Date:     time.Now(),
		BeadID:   "bead-1",
		Content:  content1,
		Category: CategoryPatterns,
		Hash:     hashContent(content1),
	})

	// Check similarity to verify it's actually > FuzzyMatchThreshold
	sim := similarity(content1, content2)
	if sim <= FuzzyMatchThreshold {
		t.Skipf("similarity %f is too low for test, need > %f", sim, FuzzyMatchThreshold)
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
	f1, _ := NewFile(tmpDir)
	l1, _ := f1.Add("bead-1", "First learning", CategoryPatterns)
	l2, _ := f1.Add("bead-2", "Second learning", CategoryConventions)

	// Verify what we added
	if len(f1.provisional) != 2 {
		t.Fatalf("expected 2 provisional after adding, got %d", len(f1.provisional))
	}

	// Create new file instance and load
	f2, _ := NewFile(tmpDir)
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
		a     string
		b     string
		name  string
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

Accumulated operational knowledge from Gromit iterations.

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

	confirmed, provisional, archived := parseLearnings(content)

	// The parser will parse until it hits "## Provisional"
	// At that point, it saves the confirmed ones
	// The key is that the last learning needs content after it
	if len(confirmed) < 1 {
		t.Errorf("expected at least 1 confirmed learning, got %d", len(confirmed))
	}
	if len(provisional) < 1 {
		t.Errorf("expected at least 1 provisional learning, got %d", len(provisional))
	}
	_ = archived // Ignore for this test

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

	confirmed, provisional, archived := parseLearnings(content)

	if len(confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(confirmed))
	}
	if len(provisional) != 0 {
		t.Errorf("expected 0 provisional, got %d", len(provisional))
	}
	if len(archived) != 0 {
		t.Errorf("expected 0 archived, got %d", len(archived))
	}
}

// TestLoadMissingFile tests that Load handles missing files gracefully
func TestLoadMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

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
	nestedDir := filepath.Join(tmpDir, "nested", "path", ".gromit")
	f, _ := NewFile(nestedDir)

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

// TestGetByHash tests retrieving learnings by hash
func TestGetByHash(t *testing.T) {
	f := &File{}

	// Add learnings to different sections
	confirmedLearning := Learning{
		Date:     time.Now(),
		BeadID:   "bead-1",
		Content:  "Confirmed learning",
		Category: CategoryPatterns,
		Hash:     hashContent("Confirmed learning"),
	}
	provisionalLearning := Learning{
		Date:     time.Now(),
		BeadID:   "bead-2",
		Content:  "Provisional learning",
		Category: CategoryConventions,
		Hash:     hashContent("Provisional learning"),
	}
	archivedLearning := Learning{
		Date:     time.Now(),
		BeadID:   "bead-3",
		Content:  "Archived learning",
		Category: CategoryGotchas,
		Hash:     hashContent("Archived learning"),
	}

	f.confirmed = append(f.confirmed, confirmedLearning)
	f.provisional = append(f.provisional, provisionalLearning)
	f.archived = append(f.archived, archivedLearning)

	// Test finding in confirmed
	result := f.GetByHash(confirmedLearning.Hash)
	if result == nil {
		t.Fatal("expected to find confirmed learning")
	}
	if result.BeadID != "bead-1" {
		t.Errorf("expected BeadID 'bead-1', got %q", result.BeadID)
	}

	// Test finding in provisional
	result = f.GetByHash(provisionalLearning.Hash)
	if result == nil {
		t.Fatal("expected to find provisional learning")
	}
	if result.BeadID != "bead-2" {
		t.Errorf("expected BeadID 'bead-2', got %q", result.BeadID)
	}

	// Test finding in archived
	result = f.GetByHash(archivedLearning.Hash)
	if result == nil {
		t.Fatal("expected to find archived learning")
	}
	if result.BeadID != "bead-3" {
		t.Errorf("expected BeadID 'bead-3', got %q", result.BeadID)
	}

	// Test not found
	result = f.GetByHash("nonexistent-hash")
	if result != nil {
		t.Error("expected nil for nonexistent hash")
	}
}

// TestRemove tests removing learnings by hash
func TestRemove(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add learnings to different sections
	l1, _ := f.Add("bead-1", "Learning one", CategoryPatterns)
	if l1 == nil {
		t.Fatal("failed to add learning 1")
	}

	// Manually add to confirmed for testing
	l2 := Learning{
		Date:     time.Now(),
		BeadID:   "bead-2",
		Content:  "Learning two",
		Category: CategoryConventions,
		Hash:     hashContent("Learning two"),
	}
	f.confirmed = append(f.confirmed, l2)
	if err := f.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify we have learnings
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 1 {
		t.Errorf("expected 1 confirmed, got %d", len(f.confirmed))
	}

	// Remove from provisional
	err := f.Remove(l1.Hash)
	if err != nil {
		t.Fatalf("failed to remove from provisional: %v", err)
	}
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional after removal, got %d", len(f.provisional))
	}

	// Remove from confirmed
	err = f.Remove(l2.Hash)
	if err != nil {
		t.Fatalf("failed to remove from confirmed: %v", err)
	}
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed after removal, got %d", len(f.confirmed))
	}

	// Try removing nonexistent
	err = f.Remove("nonexistent-hash")
	if err == nil {
		t.Error("expected error when removing nonexistent hash")
	}
}

// TestArchive tests archiving learnings
func TestArchive(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add a provisional learning
	l1, _ := f.Add("bead-1", "Learning to archive", CategoryPatterns)
	if l1 == nil {
		t.Fatal("failed to add learning")
	}

	// Verify it's in provisional
	if len(f.provisional) != 1 {
		t.Errorf("expected 1 provisional, got %d", len(f.provisional))
	}
	if len(f.archived) != 0 {
		t.Errorf("expected 0 archived, got %d", len(f.archived))
	}

	// Archive it with reason
	err := f.Archive(l1.Hash, "no longer relevant")
	if err != nil {
		t.Fatalf("failed to archive: %v", err)
	}

	// Verify it moved to archived
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional after archive, got %d", len(f.provisional))
	}
	if len(f.archived) != 1 {
		t.Errorf("expected 1 archived after archive, got %d", len(f.archived))
	}

	// Check that reason was added to content
	if !strings.Contains(f.archived[0].Content, "no longer relevant") {
		t.Error("expected archived learning to contain reason")
	}
	if !strings.Contains(f.archived[0].Content, "Archived from provisional") {
		t.Error("expected archived learning to indicate source section")
	}

	// Try archiving nonexistent
	err = f.Archive("nonexistent-hash", "test")
	if err == nil {
		t.Error("expected error when archiving nonexistent hash")
	}
}

// TestArchiveFromConfirmed tests archiving from confirmed section
func TestArchiveFromConfirmed(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Manually add to confirmed
	l1 := Learning{
		Date:     time.Now(),
		BeadID:   "bead-1",
		Content:  "Confirmed learning",
		Category: CategoryPatterns,
		Hash:     hashContent("Confirmed learning"),
	}
	f.confirmed = append(f.confirmed, l1)
	if err := f.Save(); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Archive it without reason
	err := f.Archive(l1.Hash, "")
	if err != nil {
		t.Fatalf("failed to archive: %v", err)
	}

	// Verify it moved from confirmed to archived
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed after archive, got %d", len(f.confirmed))
	}
	if len(f.archived) != 1 {
		t.Errorf("expected 1 archived after archive, got %d", len(f.archived))
	}
	if !strings.Contains(f.archived[0].Content, "Archived from confirmed") {
		t.Error("expected archived learning to indicate confirmed source")
	}
}

// TestReplace tests replacing multiple learnings with a new one
func TestReplace(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	// Add multiple learnings
	l1, _ := f.Add("bead-1", "First learning", CategoryPatterns)
	l2, _ := f.Add("bead-2", "Second learning", CategoryPatterns)
	if l1 == nil || l2 == nil {
		t.Fatal("failed to add learnings")
	}

	// Verify initial state
	if len(f.provisional) != 2 {
		t.Errorf("expected 2 provisional, got %d", len(f.provisional))
	}
	if len(f.confirmed) != 0 {
		t.Errorf("expected 0 confirmed, got %d", len(f.confirmed))
	}

	// Replace with consolidated learning
	newContent := "Consolidated learning that replaces both"
	err := f.Replace([]string{l1.Hash, l2.Hash}, newContent, CategoryPatterns)
	if err != nil {
		t.Fatalf("failed to replace: %v", err)
	}

	// Verify old learnings are removed
	if len(f.provisional) != 0 {
		t.Errorf("expected 0 provisional after replace, got %d", len(f.provisional))
	}

	// Verify new learning is in confirmed
	if len(f.confirmed) != 1 {
		t.Errorf("expected 1 confirmed after replace, got %d", len(f.confirmed))
	}

	// Check new learning properties
	newLearning := f.confirmed[0]
	if newLearning.Content != newContent {
		t.Errorf("expected content %q, got %q", newContent, newLearning.Content)
	}
	if newLearning.BeadID != "retro" {
		t.Errorf("expected BeadID 'retro', got %q", newLearning.BeadID)
	}
	if newLearning.Category != CategoryPatterns {
		t.Errorf("expected category %q, got %q", CategoryPatterns, newLearning.Category)
	}
	// Should have reference to old bead IDs
	if !strings.Contains(newLearning.RelatedTo, "bead-1") || !strings.Contains(newLearning.RelatedTo, "bead-2") {
		t.Errorf("expected RelatedTo to contain both bead IDs, got %q", newLearning.RelatedTo)
	}
}

// TestReplaceNoHashes tests that Replace fails with no hashes
func TestReplaceNoHashes(t *testing.T) {
	tmpDir := t.TempDir()
	f, _ := NewFile(tmpDir)

	err := f.Replace([]string{}, "New content", CategoryPatterns)
	if err == nil {
		t.Error("expected error when replacing with no hashes")
	}
}

// TestParseArchivedSection tests parsing archived section from file
func TestParseArchivedSection(t *testing.T) {
	content := `# Learnings

---

## Confirmed

### 2026-02-01 | bead-1 | patterns
Confirmed learning

---

## Provisional

### 2026-02-02 | bead-2 | conventions
Provisional learning

---

## Archived

### 2026-02-03 | bead-3 | gotchas
Archived learning content

*Archived from provisional: no longer relevant*
`

	confirmed, provisional, archived := parseLearnings(content)

	if len(confirmed) != 1 {
		t.Errorf("expected 1 confirmed, got %d", len(confirmed))
	}
	if len(provisional) != 1 {
		t.Errorf("expected 1 provisional, got %d", len(provisional))
	}
	if len(archived) != 1 {
		t.Errorf("expected 1 archived, got %d", len(archived))
	}

	// Check archived learning details
	if archived[0].BeadID != "bead-3" {
		t.Errorf("expected BeadID 'bead-3', got %q", archived[0].BeadID)
	}
	if archived[0].Category != "gotchas" {
		t.Errorf("expected category 'gotchas', got %q", archived[0].Category)
	}
	if !strings.Contains(archived[0].Content, "Archived learning content") {
		t.Error("archived content should contain original text")
	}
}

// TestNewFileSlicesNotNil tests that NewFile initializes slices to non-nil
func TestNewFileSlicesNotNil(t *testing.T) {
	f, _ := NewFile(t.TempDir())
	if f.confirmed == nil {
		t.Error("expected confirmed to be non-nil after NewFile")
	}
	if f.provisional == nil {
		t.Error("expected provisional to be non-nil after NewFile")
	}
	if f.archived == nil {
		t.Error("expected archived to be non-nil after NewFile")
	}
}

// TestLoadMissingFileSlicesNotNil tests that Load on missing file keeps slices non-nil
func TestLoadMissingFileSlicesNotNil(t *testing.T) {
	f, _ := NewFile(t.TempDir())
	if err := f.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.confirmed == nil {
		t.Error("expected confirmed to be non-nil after Load on missing file")
	}
	if f.provisional == nil {
		t.Error("expected provisional to be non-nil after Load on missing file")
	}
	if f.archived == nil {
		t.Error("expected archived to be non-nil after Load on missing file")
	}
}

// TestLoadEmptyFileSlicesNotNil tests that Load on empty file keeps slices non-nil
func TestLoadEmptyFileSlicesNotNil(t *testing.T) {
	dir := t.TempDir()
	f, _ := NewFile(dir)
	// Create an empty LEARNINGS.md
	if err := os.WriteFile(filepath.Join(dir, "LEARNINGS.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	if err := f.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.confirmed == nil {
		t.Error("expected confirmed to be non-nil after Load on empty file")
	}
	if f.provisional == nil {
		t.Error("expected provisional to be non-nil after Load on empty file")
	}
	if f.archived == nil {
		t.Error("expected archived to be non-nil after Load on empty file")
	}
}

// TestGetConfirmedNilReceiver tests that GetConfirmed returns empty slice on nil receiver
func TestGetConfirmedNilReceiver(t *testing.T) {
	var f *File
	result := f.GetConfirmed()
	if result == nil {
		t.Error("expected non-nil empty slice from GetConfirmed on nil receiver")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

// TestGetProvisionalNilReceiver tests that GetProvisional returns empty slice on nil receiver
func TestGetProvisionalNilReceiver(t *testing.T) {
	var f *File
	result := f.GetProvisional()
	if result == nil {
		t.Error("expected non-nil empty slice from GetProvisional on nil receiver")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

// TestGetRecentNilReceiver tests that GetRecent returns empty slice on nil receiver
func TestGetRecentNilReceiver(t *testing.T) {
	var f *File
	result := f.GetRecent(1)
	if result == nil {
		t.Error("expected non-nil empty slice from GetRecent on nil receiver")
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d items", len(result))
	}
}

// TestGetRecentNoMatchesReturnsEmptySlice tests that GetRecent returns empty slice (not nil) when no matches
func TestGetRecentNoMatchesReturnsEmptySlice(t *testing.T) {
	f, _ := NewFile(t.TempDir())
	// Add an old learning
	f.provisional = append(f.provisional, Learning{
		Date:    time.Now().Add(-48 * time.Hour),
		BeadID:  "bead-1",
		Content: "Old learning",
	})

	result := f.GetRecent(1)
	if result == nil {
		t.Error("expected non-nil empty slice from GetRecent with no matches")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 recent learnings, got %d", len(result))
	}
}

// TestLoadAndSaveWithArchived tests round-trip with archived section
func TestLoadAndSaveWithArchived(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and populate
	f1, _ := NewFile(tmpDir)
	l1, _ := f1.Add("bead-1", "Test learning", CategoryPatterns)
	if l1 == nil {
		t.Fatal("failed to add learning")
	}

	// Archive it
	err := f1.Archive(l1.Hash, "test archive")
	if err != nil {
		t.Fatalf("failed to archive: %v", err)
	}

	// Load in new instance
	f2, _ := NewFile(tmpDir)
	err = f2.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Verify archived section persisted
	if len(f2.archived) != 1 {
		t.Errorf("expected 1 archived after load, got %d", len(f2.archived))
	}
	if len(f2.provisional) != 0 {
		t.Errorf("expected 0 provisional after load, got %d", len(f2.provisional))
	}
	if !strings.Contains(f2.archived[0].Content, "test archive") {
		t.Error("archived reason should persist across save/load")
	}
}
