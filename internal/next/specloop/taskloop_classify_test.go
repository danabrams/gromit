package specloop

import (
	"strings"
	"testing"
)

func TestIsBuildCheck(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"go build", true},
		{"go build ./...", true},
		{"go build ./internal/next/planner/...", true},
		{"go vet", true},
		{"go vet ./...", true},
		{"npm run build", true},
		{"cargo build", true},
		{"mvn compile", true},
		{"make build", true},
		{"grep -q 'func Reject' internal/next/proposaltriage/promote.go", false},
		{"grep -q '--title' cmd/gromit-next/review_proposals.go", false},
		{"awk '/stepA/{ a=NR }' file.go", false},
		{"go test -run TestFoo ./...", false},
		{"./binary --help | grep -q -- '--flag'", false},
	}
	for _, c := range cases {
		got := isBuildCheck(c.cmd)
		if got != c.want {
			t.Errorf("isBuildCheck(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}

func TestAnnotateSuspectProofChecks_AllBuildPass(t *testing.T) {
	proofChecks := []string{
		"go build ./...",
		"grep -q '--title' cmd/gromit-next/review_proposals.go",
		"grep -q '--change' cmd/gromit-next/review_proposals.go",
	}
	failures := []string{
		"grep -q '--title' cmd/gromit-next/review_proposals.go: exit status 1",
		"grep -q '--change' cmd/gromit-next/review_proposals.go: exit status 1",
	}
	result := annotateSuspectProofChecks(proofChecks, failures, nil)
	if len(result) != len(failures) {
		t.Fatalf("expected %d failures, got %d", len(failures), len(result))
	}
	for _, f := range result {
		if !strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Errorf("expected suspect prefix on %q", f)
		}
	}
}

func TestAnnotateSuspectProofChecks_BuildFailing(t *testing.T) {
	proofChecks := []string{
		"go build ./...",
		"grep -q 'func Foo' internal/foo.go",
	}
	failures := []string{
		"go build ./...: exit status 1: undefined: Bar",
		"grep -q 'func Foo' internal/foo.go: exit status 1",
	}
	result := annotateSuspectProofChecks(proofChecks, failures, nil)
	for _, f := range result {
		if strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Errorf("should NOT have suspect prefix when build is also failing: %q", f)
		}
	}
}

func TestAnnotateSuspectProofChecks_NoBuildCheck(t *testing.T) {
	proofChecks := []string{
		"grep -q 'func Foo' internal/foo.go",
	}
	failures := []string{
		"grep -q 'func Foo' internal/foo.go: exit status 1",
	}
	result := annotateSuspectProofChecks(proofChecks, failures, nil)
	for _, f := range result {
		if strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Errorf("should NOT annotate when no build check exists: %q", f)
		}
	}
}

func TestAnnotateSuspectProofChecks_EmptyInputs(t *testing.T) {
	if got := annotateSuspectProofChecks(nil, []string{"fail"}, nil); strings.HasPrefix(got[0], "[suspect-proof-check]") {
		t.Error("nil proofChecks should not annotate")
	}
	if got := annotateSuspectProofChecks([]string{"go build ./..."}, nil, nil); got != nil {
		t.Error("nil failures should return nil")
	}
}

// TestAnnotateSuspectProofChecks_FalsePositiveCollision verifies that the
// structural path (non-nil checkResults) correctly annotates failures when
// the build check PASSED, even when the grep failure message incidentally
// contains the literal text of the build command. This is the false-positive
// collision that the legacy strings.Contains heuristic would misfire on.
func TestAnnotateSuspectProofChecks_FalsePositiveCollision(t *testing.T) {
	proofChecks := []string{
		"go build ./...",
		"grep -q '--title' cmd/foo.go",
	}
	// The grep failure message happens to contain the build command text.
	// Under the legacy heuristic this would suppress annotation (false negative).
	failures := []string{
		"grep output: go build ./... --flag not found",
	}
	checkResults := []ProofCheckResult{
		{Command: "go build ./...", Pass: true},
		{Command: "grep -q '--title' cmd/foo.go", Pass: false},
	}
	result := annotateSuspectProofChecks(proofChecks, failures, checkResults)
	if len(result) != 1 {
		t.Fatalf("expected 1 annotated failure, got %d", len(result))
	}
	if !strings.HasPrefix(result[0], "[suspect-proof-check]") {
		t.Errorf("expected suspect annotation when build passed (structural path), got: %q", result[0])
	}
}
