package retro

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/logger"
)

func TestRenderPromptMissingKeyZero(t *testing.T) {
	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "PROMPT_retro.md")
	content := "Missing key: {{ .BeadStats.Missing.BeadID }}"
	if err := os.WriteFile(templatePath, []byte(content), 0o600); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	beadStats := map[string]logger.BeadStats{}
	r := &Retro{templatePath: templatePath}
	got, err := r.renderPrompt("", "", logger.RunStats{}, beadStats, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("renderPrompt error: %v", err)
	}
	if want := "Missing key: "; got != want {
		t.Fatalf("renderPrompt = %q, want %q", got, want)
	}
}
