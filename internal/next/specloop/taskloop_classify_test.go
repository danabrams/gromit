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
	result := annotateSuspectProofChecks(proofChecks, failures)
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
	result := annotateSuspectProofChecks(proofChecks, failures)
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
	result := annotateSuspectProofChecks(proofChecks, failures)
	for _, f := range result {
		if strings.HasPrefix(f, "[suspect-proof-check]") {
			t.Errorf("should NOT annotate when no build check exists: %q", f)
		}
	}
}

func TestAnnotateSuspectProofChecks_EmptyInputs(t *testing.T) {
	if got := annotateSuspectProofChecks(nil, []string{"fail"}); strings.HasPrefix(got[0], "[suspect-proof-check]") {
		t.Error("nil proofChecks should not annotate")
	}
	if got := annotateSuspectProofChecks([]string{"go build ./..."}, nil); got != nil {
		t.Error("nil failures should return nil")
	}
}
