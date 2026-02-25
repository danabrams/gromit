package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterRootCommands_RegistersBenchmarkRun(t *testing.T) {
	root := &cobra.Command{Use: "gromit"}

	registerRootCommands(root)

	benchmark, _, err := root.Find([]string{"benchmark"})
	if err != nil {
		t.Fatalf("find benchmark command: %v", err)
	}
	if benchmark == nil || benchmark.Name() != "benchmark" {
		t.Fatalf("benchmark command = %v", benchmark)
	}

	run, _, err := root.Find([]string{"benchmark", "run"})
	if err != nil {
		t.Fatalf("find benchmark run command: %v", err)
	}
	if run == nil || run.Name() != "run" {
		t.Fatalf("benchmark run command = %v", run)
	}
}

func TestCommandRequiresRepoRoot_Regression(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{name: "run requires repo root", cmd: runCmd, want: true},
		{name: "status requires repo root", cmd: statusCmd, want: true},
		{name: "init does not require repo root", cmd: initCmd, want: false},
		{name: "benchmark run does not require repo root", cmd: benchmarkRunCmd, want: false},
	}

	for _, tt := range tests {
		if got := commandRequiresRepoRoot(tt.cmd); got != tt.want {
			t.Fatalf("%s: commandRequiresRepoRoot() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestCommandRequiresRepoRoot_InitAlias(t *testing.T) {
	alias := &cobra.Command{
		Use: "init",
	}

	if commandRequiresRepoRoot(alias) {
		t.Fatalf("init alias should not require repo root")
	}
}
