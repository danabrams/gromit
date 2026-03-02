package specgate

import (
	"os"
	"strings"
	"testing"
)

func TestGateVerdictNormalizeNilFieldsCommentReferencesClaude(t *testing.T) {
	data, err := os.ReadFile("verdict.go")
	if err != nil {
		t.Fatalf("failed to read verdict.go: %v", err)
	}
	if !strings.Contains(string(data), "CLAUDE.md nil-field normalization visibility") {
		t.Fatalf("expected CLAUDE policy visibility comment for GateVerdict.normalizeNilFields")
	}
}
