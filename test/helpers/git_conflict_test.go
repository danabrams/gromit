package helpers

import (
    "strings"
    "testing"
)

func TestDeterministicGitConflictFixtureProducesMergeConflict(t *testing.T) {
    fixture := NewDeterministicGitConflictFixture(t)
    if fixture == nil {
        t.Fatal("fixture must not be nil")
    }
    if fixture.OurBranch == "" || fixture.TheirBranch == "" {
        t.Fatalf("expected populated branch names, got ours=%q theirs=%q", fixture.OurBranch, fixture.TheirBranch)
    }

    if out, err := fixture.RunGit("checkout", fixture.OurBranch); err != nil {
        t.Fatalf("git checkout %s failed: %v\noutput:\n%s", fixture.OurBranch, err, out)
    }

    conflictOutput, mergeErr := fixture.RunGit("merge", fixture.TheirBranch)
    if mergeErr == nil {
        t.Fatalf("expected merge conflict, got nil error\noutput:\n%s", conflictOutput)
    }
    if !strings.Contains(conflictOutput, "CONFLICT") {
        t.Fatalf("expected conflict text in git output, got:\n%s", conflictOutput)
    }
}
