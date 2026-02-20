package runner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
)

func TestNewRunnerWiresMaxLearningCharsIntoRenderer(t *testing.T) {
	tests := []struct {
		name             string
		maxLearningChars int
		wantLearnings    int
	}{
		{
			name:             "budget cap limits confirmed learnings",
			maxLearningChars: 250,
			wantLearnings:    2,
		},
		{
			name:             "zero budget keeps backward compatible unlimited behavior",
			maxLearningChars: 0,
			wantLearnings:    4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gromitDir := filepath.Join(tmpDir, ".gromit")
			templatesDir := filepath.Join(gromitDir, "templates")
			specsDir := filepath.Join(gromitDir, "specs")
			logsDir := filepath.Join(gromitDir, "logs")
			if err := os.MkdirAll(templatesDir, 0o755); err != nil {
				t.Fatalf("MkdirAll templates: %v", err)
			}
			if err := os.MkdirAll(specsDir, 0o755); err != nil {
				t.Fatalf("MkdirAll specs: %v", err)
			}
			if err := os.MkdirAll(logsDir, 0o755); err != nil {
				t.Fatalf("MkdirAll logs: %v", err)
			}

			if err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# test"), 0o644); err != nil {
				t.Fatalf("WriteFile CLAUDE.md: %v", err)
			}
			if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("# rules"), 0o644); err != nil {
				t.Fatalf("WriteFile RULES.md: %v", err)
			}
			if err := os.WriteFile(filepath.Join(gromitDir, "LEARNINGS.md"), []byte(testLearningsMarkdown(4, 120)), 0o644); err != nil {
				t.Fatalf("WriteFile LEARNINGS.md: %v", err)
			}

			cfg := &config.Config{
				Paths: config.PathsConfig{
					Templates:       templatesDir,
					Specs:           specsDir,
					ProjectClaudeMD: filepath.Join(tmpDir, "CLAUDE.md"),
					Logs:            logsDir,
				},
				Claude: config.ClaudeConfig{
					Binary:  "claude",
					Timeout: 600,
				},
				Learnings: config.LearningsConfig{
					MaxLearningChars: tt.maxLearningChars,
				},
			}

			var out bytes.Buffer
			r, err := NewRunner(cfg, &out)
			if err != nil {
				t.Fatalf("NewRunner() failed: %v", err)
			}
			ctx, err := r.renderer.BuildContext(&bead.Bead{ID: "test-1", Title: "test"}, nil, 1, "sonnet")
			if err != nil {
				t.Fatalf("BuildContext() failed: %v", err)
			}

			if len(ctx.ConfirmedLearnings) != tt.wantLearnings {
				t.Fatalf("confirmed learnings count = %d, want %d", len(ctx.ConfirmedLearnings), tt.wantLearnings)
			}
		})
	}
}

func testLearningsMarkdown(count int, charsPerLearning int) string {
	var sb strings.Builder
	sb.WriteString("# Learnings\n\n## Confirmed\n\n")
	for i := 0; i < count; i++ {
		date := time.Date(2026, time.January, i+1, 0, 0, 0, 0, time.UTC)
		header := fmt.Sprintf("### %s | Entry %d | patterns\n\n", date.Format("2006-01-02"), i+1)
		sb.WriteString(header)
		content := "learning-" + strings.Repeat("x", charsPerLearning-len("learning-"))
		sb.WriteString(content + "\n\n")
	}
	sb.WriteString("## Provisional\n\n## Archived\n")
	return sb.String()
}
