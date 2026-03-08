package gromit

import (
	"os"
	"strings"
	"testing"
)

func TestMainRegistersDebug2Command(t *testing.T) {
	data, err := os.ReadFile("cmd/gromit/main.go")
	if err != nil {
		t.Fatalf("failed to read cmd/gromit/main.go: %v", err)
	}
	if !strings.Contains(string(data), "root.AddCommand(debug2Cmd)") {
		t.Fatalf("registerRootCommands should register debug2Cmd in main.go")
	}
}
