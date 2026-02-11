//go:build acceptance

package bead

import (
	"os"
	"strings"
	"testing"
)

// TestIntegrationCallsCorrectCommand_IncludesAllAndLimitFlags verifies that TestListWithLabel_IntegrationCallsCorrectCommand
// has been updated to include --all and --limit 0 flags in its command expectation
func TestIntegrationCallsCorrectCommand_IncludesAllAndLimitFlags(t *testing.T) {
	// Expected failure: label_integration_test.go TestListWithLabel_IntegrationCallsCorrectCommand does not yet include --all and --limit 0 flags
	//
	// This test reads the integration test file and verifies that TestListWithLabel_IntegrationCallsCorrectCommand
	// constructs a bd list command with --all and --limit 0 flags (line 256 region).

	testFile := "label_integration_test.go"
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", testFile, err)
	}

	fileContent := string(content)

	// Find the TestListWithLabel_IntegrationCallsCorrectCommand function
	funcStart := strings.Index(fileContent, "func TestListWithLabel_IntegrationCallsCorrectCommand")
	if funcStart == -1 {
		t.Fatal("Could not find TestListWithLabel_IntegrationCallsCorrectCommand function")
	}

	// Find the end of the function (next function declaration or end of file)
	funcEnd := strings.Index(fileContent[funcStart+50:], "\nfunc ")
	if funcEnd == -1 {
		funcEnd = len(fileContent)
	} else {
		funcEnd += funcStart + 50
	}

	functionBody := fileContent[funcStart:funcEnd]

	// Verify the function includes --all flag in the bd list command
	if !strings.Contains(functionBody, `"--all"`) {
		t.Error("TestListWithLabel_IntegrationCallsCorrectCommand does not include --all flag in bd list command")
	}

	// Verify the function includes --limit 0 flag in the bd list command
	if !strings.Contains(functionBody, `"--limit"`) || !strings.Contains(functionBody, `"0"`) {
		t.Error("TestListWithLabel_IntegrationCallsCorrectCommand does not include --limit 0 flag in bd list command")
	}

	// Verify --all appears in an exec.Command context
	if !strings.Contains(functionBody, "bd") && !strings.Contains(functionBody, "list") {
		t.Error("TestListWithLabel_IntegrationCallsCorrectCommand does not appear to construct a bd list command")
	}
}

// TestCommandContract_IncludesAllAndLimitFlags verifies that TestListWithLabel_CommandContract
// has been updated to include --all and --limit 0 flags in its expectedCmd
func TestCommandContract_IncludesAllAndLimitFlags(t *testing.T) {
	// Expected failure: label_integration_test.go TestListWithLabel_CommandContract does not yet include --all and --limit 0 in expectedCmd
	//
	// This test reads the integration test file and verifies that TestListWithLabel_CommandContract
	// builds expectedCmd with --sort priority --all --limit 0 flags (line 456 region).

	testFile := "label_integration_test.go"
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", testFile, err)
	}

	fileContent := string(content)

	// Find the TestListWithLabel_CommandContract function
	funcStart := strings.Index(fileContent, "func TestListWithLabel_CommandContract")
	if funcStart == -1 {
		t.Fatal("Could not find TestListWithLabel_CommandContract function")
	}

	// Find the end of the function (next function declaration or end of file)
	funcEnd := strings.Index(fileContent[funcStart+50:], "\nfunc ")
	if funcEnd == -1 {
		funcEnd = len(fileContent)
	} else {
		funcEnd += funcStart + 50
	}

	functionBody := fileContent[funcStart:funcEnd]

	// Verify the function includes --all flag in expectedCmd
	if !strings.Contains(functionBody, `"--all"`) {
		t.Error("TestListWithLabel_CommandContract does not include --all flag in expectedCmd")
	}

	// Verify the function includes --limit 0 flag in expectedCmd
	if !strings.Contains(functionBody, `"--limit"`) || !strings.Contains(functionBody, `"0"`) {
		t.Error("TestListWithLabel_CommandContract does not include --limit 0 flag in expectedCmd")
	}

	// Verify expectedCmd includes --sort priority (should already be there)
	if !strings.Contains(functionBody, `"--sort"`) || !strings.Contains(functionBody, `"priority"`) {
		t.Error("TestListWithLabel_CommandContract expectedCmd missing --sort priority flags")
	}

	// Verify the expectedCmd variable exists
	if !strings.Contains(functionBody, "expectedCmd") {
		t.Error("TestListWithLabel_CommandContract does not define expectedCmd variable")
	}
}
