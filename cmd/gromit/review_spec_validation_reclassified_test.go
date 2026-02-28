//go:build acceptance

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/test/testutil"
)

type reviewSpecValidationCase struct {
	name          string
	spec          string
	mutateFixture func(t *testing.T, tmpDir string)
	wantExitCode  int
	wantAny       []string
	wantAll       []string
}

func setupReviewSpecValidationFixture(t *testing.T) string {
	t.Helper()
	return setupReviewSpecSmokeProject(t)
}

func buildReviewSpecValidationCases() []reviewSpecValidationCase {
	return []reviewSpecValidationCase{
		{
			name:         "nonexistent spec suggests existing spec",
			spec:         "nonexistent-spec",
			wantExitCode: 1,
			wantAll:      []string{"not found", "existing-spec"},
		},
		{
			name:         "typo in spec name still returns suggestion",
			spec:         "typo",
			wantExitCode: 1,
			wantAll:      []string{"not found", "existing-spec"},
		},
		{
			name: "empty specs directory reports validation error",
			spec: "existing-spec",
			mutateFixture: func(t *testing.T, tmpDir string) {
				t.Helper()
				specsDir := filepath.Join(tmpDir, ".gromit", "specs")
				if err := os.RemoveAll(specsDir); err != nil {
					t.Fatalf("remove specs dir: %v", err)
				}
				if err := os.MkdirAll(specsDir, 0o755); err != nil {
					t.Fatalf("recreate empty specs dir: %v", err)
				}
			},
			wantExitCode: 1,
			wantAny:      []string{"no specs", "not found", "does not exist"},
		},
		{
			name: "non-markdown files are ignored for spec suggestions",
			spec: "notes",
			mutateFixture: func(t *testing.T, tmpDir string) {
				t.Helper()
				specsDir := filepath.Join(tmpDir, ".gromit", "specs")
				if err := os.Remove(filepath.Join(specsDir, "existing-spec.md")); err != nil {
					t.Fatalf("remove markdown spec: %v", err)
				}
				if err := os.WriteFile(filepath.Join(specsDir, "notes.txt"), []byte("plain text"), 0o644); err != nil {
					t.Fatalf("write non-markdown file: %v", err)
				}
			},
			wantExitCode: 1,
			wantAny:      []string{"no specs", "not found", "does not exist"},
		},
		{
			name:         "existing spec passes validation path",
			spec:         "existing-spec",
			wantExitCode: 1,
			wantAny:      []string{"no matching", "no ready", "nothing to review"},
		},
	}
}

func assertSpecValidationError(t *testing.T, stderr string, tc reviewSpecValidationCase) {
	t.Helper()

	lower := strings.ToLower(stderr)
	for _, needle := range tc.wantAll {
		if !strings.Contains(lower, strings.ToLower(needle)) {
			t.Fatalf("%s: expected stderr to contain %q, got: %s", tc.name, needle, stderr)
		}
	}

	if len(tc.wantAny) == 0 {
		return
	}

	for _, needle := range tc.wantAny {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return
		}
	}
	t.Fatalf("%s: expected stderr to contain one of %q, got: %s", tc.name, tc.wantAny, stderr)
}

func TestReviewSpecValidationScenarios_TableDriven(t *testing.T) {
	t.Parallel()

	cases := buildReviewSpecValidationCases()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := setupReviewSpecValidationFixture(t)
			if tc.mutateFixture != nil {
				tc.mutateFixture(t, tmpDir)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, stderr, exitCode, err := testutil.RunGromitHelperProcessWithStdin(
				ctx,
				binaryPath,
				tmpDir,
				nil,
				"",
				"review", "--spec", tc.spec,
			)
			if err != nil {
				t.Fatalf("%s: run gromit review --spec: %v", tc.name, err)
			}
			if exitCode != tc.wantExitCode {
				t.Fatalf("%s: exit code = %d, want %d; stderr=%s", tc.name, exitCode, tc.wantExitCode, stderr)
			}

			assertSpecValidationError(t, stderr, tc)
		})
	}
}
