package debug

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestApplySystemicFix_RecommendsHumanReviewForSystemicPatch(t *testing.T) {
    ctx := context.Background()
    tmpDir := t.TempDir()

    fragmentDir := filepath.Join(tmpDir, ".gromit", "fragments")
    if err := os.MkdirAll(fragmentDir, 0o755); err != nil {
        t.Fatalf("setup fragments dir: %v", err)
    }

    fragmentPath := filepath.Join(fragmentDir, "build.md")
    if err := os.WriteFile(fragmentPath, []byte("old\n"), 0o644); err != nil {
        t.Fatalf("write fragment: %v", err)
    }

    patch := strings.Join([]string{
        "diff --git a/.gromit/fragments/build.md b/.gromit/fragments/build.md",
        "--- a/.gromit/fragments/build.md",
        "+++ b/.gromit/fragments/build.md",
        "@@ -1 +1 @@",
        "-old",
        "+new",
        "",
    }, "\n")

    req := &SystemicFixInput{
        FixCtx: &FixContext{
            WorktreeRoot: tmpDir,
            CodePatch:    patch,
            ErrorMsg:     "Prompt fragment is ambiguous and needs clarity",
        },
        RootCause:     RootCauseUnclearBead,
        FailureSignal: "Prompt fragment is ambiguous and needs clarity",
    }

    result, err := ApplySystemicFix(ctx, req)
    if err != nil {
        t.Fatalf("ApplySystemicFix() error = %v", err)
    }
    if result == nil {
        t.Fatalf("result = nil")
    }
    if !result.Applied {
        t.Fatalf("expected fix to be applied")
    }
    if result.SystemicRecommendation == "" {
        t.Fatalf("systemic recommendation empty, want guidance")
    }
    if !strings.Contains(strings.ToLower(result.SystemicRecommendation), "human review") {
        t.Fatalf("recommendation = %q, want human-review reminder", result.SystemicRecommendation)
    }

    updated, err := os.ReadFile(fragmentPath)
    if err != nil {
        t.Fatalf("read fragment: %v", err)
    }
    if !strings.Contains(string(updated), "new") {
        t.Fatalf("fragment not updated, got %q", string(updated))
    }
}
