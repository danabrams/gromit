package main

import (
	"testing"
)

func TestExecCmd_RequiresSpecFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--project", "my-project"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --spec flag")
	}
}

func TestExecCmd_RequiresProjectFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no --project flag")
	}
}

func TestExecCmd_AcceptsBothFlags(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md", "--project", "my-project"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecCmd_DryRunFlag(t *testing.T) {
	cmd := newExecSpecCmd()
	cmd.SetArgs([]string{"--spec", "./specs/spec-0002.md", "--project", "my-project", "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	if !dryRun {
		t.Fatal("expected dry-run to be true")
	}
}
