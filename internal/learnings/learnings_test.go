package learnings

import (
	"testing"
	"time"
)

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
