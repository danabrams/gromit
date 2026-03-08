package main

import (
	"strings"
	"testing"
)

func TestMainRegisterRootCommandsIncludesDebug(t *testing.T) {
	t.Parallel()

	source := mustReadText(t, "cmd/gromit/main.go")
	if !strings.Contains(source, "root.AddCommand(debugCmd)") {
		t.Fatalf("registerRootCommands should register debugCmd in main.go")
	}
}
