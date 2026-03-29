package gap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func TestReadChangedFilesFromDiff(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	diffPath := filepath.Join(tmp, "gap-analysis.diff")
	content := "internal/foo/foo.go\n\ninternal/bar/bar.go\n"
	if err := os.WriteFile(diffPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write diff file: %v", err)
	}

	changed, err := ReadChangedFiles(diffPath)
	if err != nil {
		t.Fatalf("ReadChangedFiles returned error: %v", err)
	}

	if len(changed) != 2 {
		t.Fatalf("got %d changed files, want 2", len(changed))
	}

	if changed[0] != "internal/foo/foo.go" {
		t.Fatalf("unexpected first file %q", changed[0])
	}
	if changed[1] != "internal/bar/bar.go" {
		t.Fatalf("unexpected second file %q", changed[1])
	}
}

func TestFlagChangedBeads_FlagsOverlappingBeads(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-a"},
		{ID: "bead-b"},
		{ID: "bead-c"},
	}
	changedFiles := []string{"internal/foo/foo.go", "internal/bar/bar.go"}
	beadFiles := map[string][]string{
		"bead-a": {"internal/foo/foo.go"},
		"bead-b": {"internal/qux/qux.go"},
		"bead-c": {"internal/bar/bar.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)
	want := map[string]bool{"bead-a": true, "bead-c": true}
	for _, b := range flagged {
		if _, ok := want[b.ID]; ok {
			delete(want, b.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing flagged beads: %v", want)
	}
}

func TestFlagChangedBeads_NoOverlapReturnsEmpty(t *testing.T) {
	t.Parallel()

	beads := []*bead.Bead{
		{ID: "bead-x"},
	}
	changedFiles := []string{"internal/other/other.go"}
	beadFiles := map[string][]string{
		"bead-x": {"internal/alpha/alpha.go"},
	}

	flagged := FlagChangedBeads(beads, changedFiles, beadFiles)
	if len(flagged) != 0 {
		t.Fatalf("got %d flagged beads, want 0", len(flagged))
	}
}
