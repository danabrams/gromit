package runtypes

import (
    "os"
    "strings"
    "testing"
)

func TestSubTaskNormalizeNilFieldsCommentReferencesClaude(t *testing.T) {
    data, err := os.ReadFile("types.go")
    if err != nil {
        t.Fatalf("failed to read types.go: %v", err)
    }
    if !strings.Contains(string(data), "CLAUDE.md nil-field normalization visibility") {
        t.Fatalf("expected CLAUDE policy visibility comment for SubTask.NormalizeNilFields")
    }
}
