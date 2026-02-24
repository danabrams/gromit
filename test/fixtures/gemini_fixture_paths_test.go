package fixtures_test

import (
	"path/filepath"
	"testing"
)

func TestGeminiFixturePath_UsesCanonicalDirectory(t *testing.T) {
	got := geminiFixturePath("commands.log")
	want := filepath.Join("..", "..", "test", "fixtures", "gemini", "commands.log")
	if got != want {
		t.Fatalf("geminiFixturePath() = %q, want %q", got, want)
	}
}
