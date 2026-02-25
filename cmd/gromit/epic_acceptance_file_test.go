package main

import (
    "os"
    "testing"
)

func TestEpicAcceptanceFileExists(t *testing.T) {
    const acceptancePath = "cmd/gromit/epic_acceptance_test.go"
    info, err := os.Stat(acceptancePath)
    if err != nil {
        t.Fatalf("acceptance test file %s missing: %v", acceptancePath, err)
    }
    if info.IsDir() {
        t.Fatalf("expected %s to be a file", acceptancePath)
    }
}
