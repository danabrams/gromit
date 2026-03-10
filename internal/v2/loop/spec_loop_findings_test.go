package loop

import (
	"reflect"
	"testing"

	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestConvertSpecFindingsPreservesAffectedFiles(t *testing.T) {
	t.Parallel()

	src := []stagepkg.SpecFinding{{
		Title:         "affected files",
		Description:   "test",
		Severity:      stagepkg.SpecFindingSeverityHigh,
		Category:      stagepkg.SpecFindingCategoryQuality,
		Scope:         stagepkg.SpecFindingScopeSpec,
		AffectedFiles: []string{"file1.go", "file2.go"},
	}}

	got := convertSpecFindings(src)
	if len(got) != 1 {
		t.Fatalf("convertSpecFindings returned %d entries, want 1", len(got))
	}

	want := []string{"file1.go", "file2.go"}
	if !reflect.DeepEqual(got[0].AffectedFiles, want) {
		t.Fatalf("affected files = %#v, want %#v", got[0].AffectedFiles, want)
	}
}
