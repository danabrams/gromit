package main

import "testing"

func TestExperimentsCmd_RegistrationAndFlags(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "experiments" {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("experiments command not registered")
	}

	jsonFlag := experimentsCmd.Flags().Lookup("json")
	if jsonFlag == nil {
		t.Fatalf("experiments command should expose --json flag")
	}
	if jsonFlag.Value.Type() != "bool" {
		t.Fatalf("--json flag should be bool, got %s", jsonFlag.Value.Type())
	}
}
