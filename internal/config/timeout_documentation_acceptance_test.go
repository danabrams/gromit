//go:build acceptance

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProjectGromitYAML_OpusTimeoutOverrides verifies that gromit.yaml includes
// opus timeout overrides with explanatory comments.
func TestProjectGromitYAML_OpusTimeoutOverrides(t *testing.T) {
	// Expected failure: gromit.yaml does not currently have opus model_timeouts entry
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", cfgPath, err)
	}

	t.Run("opus_entry_exists", func(t *testing.T) {
		// Expected failure: opus key does not exist in model_timeouts map yet
		opus, ok := cfg.Claude.ModelTimeouts["opus"]
		if !ok {
			t.Fatal("gromit.yaml missing model_timeouts entry for opus")
		}

		// Verify opus has at least one non-zero override
		if opus.Timeout == 0 && opus.StallTimeout == 0 && opus.StallTimeoutActive == 0 && opus.BeadTimeout == 0 {
			t.Error("opus model_timeouts entry exists but has no non-zero overrides")
		}
	})
}

// TestProjectGromitYAML_ModelTimeoutComments verifies that each model timeout
// override has an explanatory comment in the YAML explaining the rationale.
func TestProjectGromitYAML_ModelTimeoutComments(t *testing.T) {
	// Expected failure: comments explaining timeout rationale do not exist in gromit.yaml yet
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	yamlContent, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", cfgPath, err)
	}

	content := string(yamlContent)

	t.Run("sonnet_timeout_has_rationale", func(t *testing.T) {
		if !strings.Contains(content, "model_timeouts:") {
			t.Fatal("gromit.yaml missing model_timeouts section")
		}

		sonnetIdx := strings.Index(content, "sonnet:")
		if sonnetIdx == -1 {
			t.Fatal("gromit.yaml missing sonnet entry in model_timeouts")
		}

		lineContent := extractLineWithField(content, sonnetIdx, "timeout:")
		if lineContent == "" {
			t.Fatal("sonnet entry missing timeout field")
		}

		if !strings.Contains(lineContent, "#") {
			t.Error("sonnet timeout field missing explanatory comment")
		}

		if commentIdx := strings.Index(lineContent, "#"); commentIdx != -1 {
			comment := lineContent[commentIdx:]
			if !containsRationale(comment) {
				t.Errorf("sonnet timeout comment does not explain rationale: %s", comment)
			}
		}
	})

	t.Run("opus_timeout_has_rationale", func(t *testing.T) {
		opusIdx := strings.Index(content, "opus:")
		if opusIdx == -1 {
			t.Fatal("gromit.yaml missing opus entry in model_timeouts")
		}

		// opus might not have timeout override - check for any timeout field
		lineContent := extractLineWithField(content, opusIdx, "timeout:")
		if lineContent == "" {
			if !strings.Contains(content[opusIdx:], "stall_timeout:") &&
				!strings.Contains(content[opusIdx:], "bead_timeout:") {
				t.Skip("opus entry has no timeout overrides, skipping comment check")
			}
			return
		}

		if !strings.Contains(lineContent, "#") {
			t.Error("opus timeout field missing explanatory comment")
		}

		if commentIdx := strings.Index(lineContent, "#"); commentIdx != -1 {
			comment := lineContent[commentIdx:]
			if !containsRationale(comment) {
				t.Errorf("opus timeout comment does not explain rationale: %s", comment)
			}
		}
	})

	t.Run("haiku_timeout_has_rationale", func(t *testing.T) {
		haikuIdx := strings.Index(content, "haiku:")
		if haikuIdx == -1 {
			t.Fatal("gromit.yaml missing haiku entry in model_timeouts")
		}

		// Check stall_timeout comment (haiku's primary override)
		lineContent := extractLineWithField(content, haikuIdx, "stall_timeout:")
		if lineContent == "" {
			t.Skip("haiku has no stall_timeout override, skipping comment check")
		}

		if !strings.Contains(lineContent, "#") {
			t.Error("haiku stall_timeout field missing explanatory comment")
		}

		if commentIdx := strings.Index(lineContent, "#"); commentIdx != -1 {
			comment := lineContent[commentIdx:]
			if !containsRationale(comment) {
				t.Errorf("haiku stall_timeout comment does not explain rationale: %s", comment)
			}
		}
	})
}

// extractLineWithField finds the line containing fieldName after startIdx and returns it.
// Returns empty string if field not found.
func extractLineWithField(content string, startIdx int, fieldName string) string {
	fieldIdx := strings.Index(content[startIdx:], fieldName)
	if fieldIdx == -1 {
		return ""
	}

	// Find line boundaries
	absFieldIdx := startIdx + fieldIdx
	lineStart := strings.LastIndex(content[:absFieldIdx], "\n")
	lineEnd := strings.Index(content[absFieldIdx:], "\n")
	if lineEnd == -1 {
		lineEnd = len(content) - absFieldIdx
	}

	return content[lineStart : absFieldIdx+lineEnd]
}

// containsRationale checks if a comment explains WHY a timeout is set, not just WHAT it is
func containsRationale(comment string) bool {
	lowerComment := strings.ToLower(comment)

	// Rationale keywords that explain reasoning
	rationaleKeywords := []string{
		"needs", "requires", "consistently", "deeper", "complex",
		"longer", "shorter", "quickly", "slowly", "allow", "prevent",
		"because", "since", "so that", "to avoid", "to ensure",
	}

	for _, keyword := range rationaleKeywords {
		if strings.Contains(lowerComment, keyword) {
			return true
		}
	}

	return false
}

// TestProjectGromitYAML_OpusTimeoutRecommendation verifies that opus timeout
// overrides, if present, are higher than sonnet's given opus handles more complex work.
func TestProjectGromitYAML_OpusTimeoutRecommendation(t *testing.T) {
	// Expected failure: opus entry does not exist yet in model_timeouts
	projectRoot := findProjectRoot(t)
	cfgPath := filepath.Join(projectRoot, "gromit.yaml")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", cfgPath, err)
	}

	sonnet, sonnetOK := cfg.Claude.ModelTimeouts["sonnet"]
	opus, opusOK := cfg.Claude.ModelTimeouts["opus"]

	if !sonnetOK {
		t.Fatal("sonnet entry missing, cannot compare with opus")
	}
	if !opusOK {
		t.Fatal("opus entry missing in model_timeouts")
	}

	t.Run("opus_timeout_not_shorter_than_sonnet", func(t *testing.T) {
		// Expected failure: opus timeout will not exist until implementation
		// If opus has a timeout override, it should be >= sonnet's
		if opus.Timeout > 0 && sonnet.Timeout > 0 {
			if opus.Timeout < sonnet.Timeout {
				t.Errorf("opus timeout (%d) should be >= sonnet timeout (%d) since opus handles more complex work",
					opus.Timeout, sonnet.Timeout)
			}
		}
	})

	t.Run("opus_bead_timeout_not_shorter_than_sonnet", func(t *testing.T) {
		// Expected failure: opus bead_timeout will not exist until implementation
		// If opus has a bead timeout override, it should be >= sonnet's
		if opus.BeadTimeout > 0 && sonnet.BeadTimeout > 0 {
			if opus.BeadTimeout < sonnet.BeadTimeout {
				t.Errorf("opus bead_timeout (%d) should be >= sonnet bead_timeout (%d) since opus handles more complex work",
					opus.BeadTimeout, sonnet.BeadTimeout)
			}
		}
	})
}
