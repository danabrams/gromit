package gromit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNormalizeNilFieldsVisibilityPolicy validates that:
// 1. All NormalizeNilFields methods (exported and unexported) have CLAUDE comments
// 2. The comments reference the nil-field normalization visibility convention
// 3. Exported types have exported methods, unexported-scope types have unexported methods
func TestNormalizeNilFieldsVisibilityPolicy(t *testing.T) {
	const (
		clauseMarker     = "CLAUDE.md"
		conventionMarker = "nil-field normalization visibility convention"
	)

	// Expected methods by file - format: "FileName" -> [(methodName, isExported), ...]
	expectedMethods := map[string][]struct {
		method     string
		isExported bool
	}{
		// Exported methods (cross-package boundary types)
		"types.go": {
			{"NormalizeNilFields", true}, // SubTask
		},
		"config_normalize.go": {
			{"NormalizeNilFields", true}, // Config
		},
		"interactive_state.go": {
			{"NormalizeNilFields", true}, // InteractiveState
		},
		"state.go": {
			{"NormalizeNilFields", true}, // State
		},

		// Unexported methods (internal-only types)
		"verdict.go": {
			{"normalizeNilFields", false}, // GateVerdict
		},
		"context_types.go": {
			{"normalizeNilFields", false}, // Context, ScopeEstimate
		},
		"bead.go": {
			{"normalizeNilFields", false}, // Bead
		},
		"review.go": {
			{"normalizeNilFields", false}, // ReviewResult
		},
		"proposals.go": {
			{"normalizeNilFields", false}, // Proposals, ConsolidationProposal
		},
		"learnings.go": {
			{"normalizeNilFields", false}, // File
		},
		"logger.go": {
			{"normalizeNilFields", false}, // BeadStats
		},
		"stream.go": {
			{"normalizeNilFields", false}, // StreamMessage
		},
		"process_trend.go": {
			{"normalizeNilFields", false}, // ProcessTrend
		},
		"validator.go": {
			{"normalizeNilFields", false}, // SelfReport
		},
	}

	// Walk internal directory to find all normalize methods
	var foundMethods map[string]map[string]bool = make(map[string]map[string]bool)

	err := filepath.Walk("internal", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		fileContent := string(content)
		fileName := filepath.Base(path)

		// Check for NormalizeNilFields and normalizeNilFields methods
		methodRegex := regexp.MustCompile(`func\s*\([^)]+\)\s*(NormalizeNilFields|normalizeNilFields)\(\)`)
		if !methodRegex.MatchString(fileContent) {
			return nil // No normalize methods in this file
		}

		if foundMethods[fileName] == nil {
			foundMethods[fileName] = make(map[string]bool)
		}

		// Check for both exported and unexported methods
		exportedRegex := regexp.MustCompile(`func\s*\([^)]+\)\s*NormalizeNilFields\(\)`)
		unexportedRegex := regexp.MustCompile(`func\s*\([^)]+\)\s*normalizeNilFields\(\)`)

		if exportedRegex.MatchString(fileContent) {
			foundMethods[fileName]["NormalizeNilFields"] = true
		}
		if unexportedRegex.MatchString(fileContent) {
			foundMethods[fileName]["normalizeNilFields"] = true
		}

		// Check for CLAUDE comments
		claudeCommentRegex := regexp.MustCompile(strings.ReplaceAll(clauseMarker, ".", `\.`))
		if !claudeCommentRegex.MatchString(fileContent) {
			t.Errorf("File %s: missing CLAUDE reference in file (should reference CLAUDE.md in comments)", path)
		}

		// Check for convention marker in comments
		lines := strings.Split(fileContent, "\n")
		for i, line := range lines {
			if strings.Contains(line, "func") && (strings.Contains(line, "NormalizeNilFields") || strings.Contains(line, "normalizeNilFields")) {
				// Look backward for comment block
				hasClaudeComment := false
				for j := i - 1; j >= 0 && j >= i-10; j-- {
					if strings.Contains(lines[j], clauseMarker) && strings.Contains(lines[j], conventionMarker) {
						hasClaudeComment = true
						break
					}
				}
				if !hasClaudeComment {
					t.Errorf("File %s: line %d - missing CLAUDE comment referencing nil-field normalization visibility convention", path, i+1)
				}
				break
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Error walking directory: %v", err)
	}

	// Verify all expected methods were found
	for fileName, methods := range expectedMethods {
		found := foundMethods[fileName]
		if found == nil {
			t.Errorf("File %s: not found or has no normalize methods", fileName)
			continue
		}

		for _, expected := range methods {
			if !found[expected.method] {
				visibility := "exported"
				if !expected.isExported {
					visibility = "unexported"
				}
				t.Errorf("File %s: expected %s method %s not found", fileName, visibility, expected.method)
			}
		}
	}
}
