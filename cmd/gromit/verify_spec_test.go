package main

import "testing"

func TestVerifySpecCmd_Registration(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "verify-spec <spec>" {
			found = true
			break
		}
	}

	if !found {
		t.Error("verify-spec command not registered with rootCmd")
	}
}

func TestVerifySpecCmd_Flags(t *testing.T) {
	flag := verifySpecCmd.Flags().Lookup("create-beads")
	if flag == nil {
		t.Error("verify-spec should have --create-beads flag")
		return
	}
	if flag.Value.Type() != "bool" {
		t.Errorf("--create-beads flag should be bool, got %s", flag.Value.Type())
	}
}
