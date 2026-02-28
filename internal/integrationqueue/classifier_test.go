package integrationqueue

import (
	"testing"
)

func TestClassifyLane_SafeLaneForMetadataOnlyPaths(t *testing.T) {
	files := []string{
		".gromit/config.yaml",
		".gromit/state.json",
		"docs/README.md",
		"docs/api.md",
		"specs/design.md",
	}

	lane := ClassifyLane(files)
	if lane != SafeLane {
		t.Fatalf("ClassifyLane(%v) = %s, want %s", files, lane, SafeLane)
	}
}

func TestClassifyLane_CodeLaneForSourcePaths(t *testing.T) {
	files := []string{
		"cmd/gromit/main.go",
		"internal/runner/runner.go",
		"go.mod",
		"Makefile",
	}

	lane := ClassifyLane(files)
	if lane != CodeLane {
		t.Fatalf("ClassifyLane(%v) = %s, want %s", files, lane, CodeLane)
	}
}

func TestClassifyLane_CodeLaneForTestPaths(t *testing.T) {
	files := []string{
		"internal/pipeline/pipeline_test.go",
		"cmd/gromit/main_test.go",
	}

	lane := ClassifyLane(files)
	if lane != CodeLane {
		t.Fatalf("ClassifyLane(%v) = %s, want %s", files, lane, CodeLane)
	}
}

func TestClassifyLane_CodeLaneForMixedMetadataAndSource(t *testing.T) {
	files := []string{
		".gromit/config.yaml",
		"docs/README.md",
		"cmd/gromit/main.go",
	}

	lane := ClassifyLane(files)
	if lane != CodeLane {
		t.Fatalf("ClassifyLane(%v) = %s, want %s", files, lane, CodeLane)
	}
}

func TestClassifyLane_SafeLaneForEmptyFileList(t *testing.T) {
	lane := ClassifyLane([]string{})
	if lane != SafeLane {
		t.Fatalf("ClassifyLane([]) = %s, want %s", lane, SafeLane)
	}
}

func TestClassifyLane_IgnoresCommandOrigin(t *testing.T) {
	// This test verifies that command origin does not affect classification.
	// Both "debug", "review", and "retro" should produce same result with same files.
	files := []string{"docs/test.md"}

	lane := ClassifyLane(files)
	if lane != SafeLane {
		t.Fatalf("ClassifyLane(%v) = %s, want %s", files, lane, SafeLane)
	}
	// The function signature doesn't take command origin, confirming it's not used
}

func TestClassifyLane_CodeLaneForVariousBuildAndConfigFiles(t *testing.T) {
	testCases := []struct {
		name  string
		files []string
	}{
		{
			name:  "go.mod",
			files: []string{"go.mod"},
		},
		{
			name:  "go.sum",
			files: []string{"go.sum"},
		},
		{
			name:  "Makefile",
			files: []string{"Makefile"},
		},
		{
			name:  "Dockerfile",
			files: []string{"Dockerfile"},
		},
		{
			name:  "nested source",
			files: []string{"pkg/util/helper.go"},
		},
		{
			name:  "vendor directory",
			files: []string{"vendor/github.com/some/lib/file.go"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			lane := ClassifyLane(tc.files)
			if lane != CodeLane {
				t.Fatalf("ClassifyLane(%v) = %s, want %s", tc.files, lane, CodeLane)
			}
		})
	}
}

func TestClassifyLane_SafeLaneForNestedMetadataPaths(t *testing.T) {
	testCases := []struct {
		name  string
		files []string
	}{
		{
			name:  "nested .gromit",
			files: []string{".gromit/state/queue.json"},
		},
		{
			name:  "nested docs",
			files: []string{"docs/api/endpoints.md", "docs/guide/setup.md"},
		},
		{
			name:  "nested specs",
			files: []string{"specs/integration/flow.md", "specs/api/schema.md"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			lane := ClassifyLane(tc.files)
			if lane != SafeLane {
				t.Fatalf("ClassifyLane(%v) = %s, want %s", tc.files, lane, SafeLane)
			}
		})
	}
}
