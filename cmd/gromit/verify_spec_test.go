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
