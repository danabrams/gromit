package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

// setupRendererWithLearnings creates a Renderer with a learnings file
// containing N confirmed learnings of known sizes. Each learning has
// content of approximately targetCharLen characters. Learnings are
// ordered oldest-first (i=0 oldest, i=n-1 most recent).
func setupRendererWithLearnings(t *testing.T, n int, targetCharLen int) *Renderer {
	t.Helper()
	tmpDir := t.TempDir()

	// Create LEARNINGS.md with confirmed learnings
	var sb strings.Builder
	sb.WriteString("# Learnings\n\n## Confirmed\n\n")
	for i := 0; i < n; i++ {
		prefix := "Learning content for entry number "
		padding := strings.Repeat("x", targetCharLen-len(prefix)-5)
		content := prefix + padding
		date := time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC)
		sb.WriteString("### " + date.Format("2006-01-02") + " | Entry " + string(rune('A'+i)) + " | patterns\n\n")
		sb.WriteString(content + "\n\n")
	}
	sb.WriteString("## Provisional\n\n## Archived\n")

	os.WriteFile(filepath.Join(tmpDir, "LEARNINGS.md"), []byte(sb.String()), 0644)

	lf, err := learnings.NewFile(tmpDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}
	if err := lf.Load(); err != nil {
		t.Fatalf("loading learnings file: %v", err)
	}

	// Create minimal template and CLAUDE.md so BuildContext doesn't error
	os.MkdirAll(filepath.Join(tmpDir, "templates"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "RULES.md"), []byte("# Rules"), 0644)

	return &Renderer{
		templatesDir:  filepath.Join(tmpDir, "templates"),
		specsDir:      filepath.Join(tmpDir, "specs"),
		claudeMDPath:  filepath.Join(tmpDir, "CLAUDE.md"),
		rulesPath:     filepath.Join(tmpDir, "RULES.md"),
		gromitDir:     tmpDir,
		learningsFile: lf,
		specCache:     make(map[string]string),
	}
}

func TestBuildContext_LearningsCapApplied(t *testing.T) {
	// Expected failure: SetMaxLearningChars method does not exist on Renderer yet.
	// BuildContext currently calls GetConfirmed() (returning all learnings),
	// not GetConfirmedFiltered() with a character budget.

	testBead := &bead.Bead{
		ID:              "test-1",
		Title:           "Test bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	tests := []struct {
		name             string
		numLearnings     int
		charPerLearning  int
		maxLearningChars int
		wantMaxCount     int // upper bound on returned learnings count
		wantMinCount     int // lower bound (at least this many should fit)
	}{
		{
			name:             "budget caps learnings to most recent subset",
			numLearnings:     5,
			charPerLearning:  100,
			maxLearningChars: 250,
			wantMaxCount:     2,
			wantMinCount:     2,
		},
		{
			name:             "zero budget means no cap returns all",
			numLearnings:     5,
			charPerLearning:  100,
			maxLearningChars: 0,
			wantMaxCount:     5,
			wantMinCount:     5,
		},
		{
			name:             "large budget returns all learnings",
			numLearnings:     3,
			charPerLearning:  100,
			maxLearningChars: 10000,
			wantMaxCount:     3,
			wantMinCount:     3,
		},
		{
			name:             "budget smaller than any single entry returns zero",
			numLearnings:     3,
			charPerLearning:  200,
			maxLearningChars: 50,
			wantMaxCount:     0,
			wantMinCount:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := setupRendererWithLearnings(t, tt.numLearnings, tt.charPerLearning)
			r.SetMaxLearningChars(tt.maxLearningChars)

			ctx, err := r.BuildContext(testBead, nil, 1, "sonnet")
			if err != nil {
				t.Fatalf("BuildContext() error = %v", err)
			}

			count := len(ctx.ConfirmedLearnings)
			if count < tt.wantMinCount {
				t.Errorf("got %d confirmed learnings, want at least %d", count, tt.wantMinCount)
			}
			if count > tt.wantMaxCount {
				t.Errorf("got %d confirmed learnings, want at most %d", count, tt.wantMaxCount)
			}
		})
	}
}

func TestBuildContext_LearningsCapPrefersMostRecent(t *testing.T) {
	// Expected failure: SetMaxLearningChars method does not exist on Renderer yet.
	// After implementation, BuildContext should use GetConfirmedFiltered which
	// selects from most recent to oldest.

	r := setupRendererWithLearnings(t, 5, 100)
	r.SetMaxLearningChars(250) // Budget fits ~2 entries of 100 chars each

	testBead := &bead.Bead{
		ID:              "test-1",
		Title:           "Test bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, err := r.BuildContext(testBead, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}

	// With 5 learnings of ~100 chars each and a 250 char budget,
	// only the 2 most recent should be returned.
	if len(ctx.ConfirmedLearnings) != 2 {
		t.Fatalf("expected 2 learnings within budget, got %d", len(ctx.ConfirmedLearnings))
	}

	// The returned learnings should be the most recent ones (newest dates).
	// Learnings are created with dates Jan 1-5; the two most recent are Jan 4 and Jan 5.
	for _, l := range ctx.ConfirmedLearnings {
		if l.Date.Before(time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("expected only most recent learnings, got one dated %v", l.Date)
		}
	}
}

func TestBuildContext_ZeroBudgetBackwardCompatible(t *testing.T) {
	// Expected failure: SetMaxLearningChars method does not exist on Renderer yet.
	// When maxLearningChars is 0 (zero-value / unset), BuildContext should
	// return all confirmed learnings, preserving backward compatibility.

	r := setupRendererWithLearnings(t, 5, 100)
	// Explicitly set zero to confirm backward-compatible behavior
	r.SetMaxLearningChars(0)

	testBead := &bead.Bead{
		ID:              "test-1",
		Title:           "Test bead",
		Priority:        1,
		Labels:          []string{},
		ExpectedOutputs: []string{},
	}

	ctx, err := r.BuildContext(testBead, nil, 1, "sonnet")
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}

	// Zero budget means no cap — all 5 learnings should be returned
	if len(ctx.ConfirmedLearnings) != 5 {
		t.Errorf("expected all 5 learnings with zero budget, got %d", len(ctx.ConfirmedLearnings))
	}
}
