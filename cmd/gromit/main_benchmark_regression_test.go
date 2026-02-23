package main

import "testing"

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
