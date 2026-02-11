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
		// Expected failure: no comment explaining why sonnet needs longer timeout
		if !strings.Contains(content, "model_timeouts:") {
			t.Fatal("gromit.yaml missing model_timeouts section")
		}

		// Find the sonnet section
		sonnetIdx := strings.Index(content, "sonnet:")
		if sonnetIdx == -1 {
			t.Fatal("gromit.yaml missing sonnet entry in model_timeouts")
		}

		// Look for timeout field and verify it has a comment explaining rationale
		timeoutIdx := strings.Index(content[sonnetIdx:], "timeout:")
		if timeoutIdx == -1 {
			t.Fatal("sonnet entry missing timeout field")
		}

		// Check for explanatory comment on the same line or line before
		lineStart := strings.LastIndex(content[:sonnetIdx+timeoutIdx], "\n")
		lineEnd := strings.Index(content[sonnetIdx+timeoutIdx:], "\n")
		if lineEnd == -1 {
			lineEnd = len(content) - (sonnetIdx + timeoutIdx)
		}
		lineContent := content[lineStart : sonnetIdx+timeoutIdx+lineEnd]

		// Verify comment exists explaining the rationale
		if !strings.Contains(lineContent, "#") {
			t.Error("sonnet timeout field missing explanatory comment")
		}

		// Verify comment contains meaningful rationale (not just field name)
		if strings.Contains(lineContent, "#") {
			commentIdx := strings.Index(lineContent, "#")
			comment := lineContent[commentIdx:]
			// Expected failure: no rationale like "consistently needs >900s" or "deeper thinking"
			if !containsRationale(comment) {
				t.Errorf("sonnet timeout comment does not explain rationale: %s", comment)
			}
		}
	})

	t.Run("opus_timeout_has_rationale", func(t *testing.T) {
		// Expected failure: opus entry does not exist yet, so no comment exists
		opusIdx := strings.Index(content, "opus:")
		if opusIdx == -1 {
			t.Fatal("gromit.yaml missing opus entry in model_timeouts")
		}

		// Look for timeout field and verify it has a comment explaining rationale
		timeoutIdx := strings.Index(content[opusIdx:], "timeout:")
		if timeoutIdx == -1 {
			// opus might not have timeout override, that's OK - check for any field
			if !strings.Contains(content[opusIdx:], "stall_timeout:") &&
				!strings.Contains(content[opusIdx:], "bead_timeout:") {
				t.Skip("opus entry has no timeout overrides, skipping comment check")
			}
			return
		}

		// Check for explanatory comment
		lineStart := strings.LastIndex(content[:opusIdx+timeoutIdx], "\n")
		lineEnd := strings.Index(content[opusIdx+timeoutIdx:], "\n")
		if lineEnd == -1 {
			lineEnd = len(content) - (opusIdx + timeoutIdx)
		}
		lineContent := content[lineStart : opusIdx+timeoutIdx+lineEnd]

		if !strings.Contains(lineContent, "#") {
			t.Error("opus timeout field missing explanatory comment")
		}

		// Verify comment contains meaningful rationale
		if strings.Contains(lineContent, "#") {
			commentIdx := strings.Index(lineContent, "#")
			comment := lineContent[commentIdx:]
			// Expected failure: no rationale explaining why opus needs specific timeout
			if !containsRationale(comment) {
				t.Errorf("opus timeout comment does not explain rationale: %s", comment)
			}
		}
	})

	t.Run("haiku_timeout_has_rationale", func(t *testing.T) {
		// Expected failure: existing haiku comments may not explain rationale sufficiently
		haikuIdx := strings.Index(content, "haiku:")
		if haikuIdx == -1 {
			t.Fatal("gromit.yaml missing haiku entry in model_timeouts")
		}

		// Check stall_timeout comment (haiku's primary override)
		stallIdx := strings.Index(content[haikuIdx:], "stall_timeout:")
		if stallIdx == -1 {
			t.Skip("haiku has no stall_timeout override, skipping comment check")
		}

		lineStart := strings.LastIndex(content[:haikuIdx+stallIdx], "\n")
		lineEnd := strings.Index(content[haikuIdx+stallIdx:], "\n")
		if lineEnd == -1 {
			lineEnd = len(content) - (haikuIdx + stallIdx)
		}
		lineContent := content[lineStart : haikuIdx+stallIdx+lineEnd]

		if !strings.Contains(lineContent, "#") {
			t.Error("haiku stall_timeout field missing explanatory comment")
		}

		// Verify comment contains meaningful rationale
		if strings.Contains(lineContent, "#") {
			commentIdx := strings.Index(lineContent, "#")
			comment := lineContent[commentIdx:]
			// Expected failure: may not explain why haiku should respond quickly
			if !containsRationale(comment) {
				t.Errorf("haiku stall_timeout comment does not explain rationale: %s", comment)
			}
		}
	})
}

// containsRationale checks if a comment explains WHY a timeout is set, not just WHAT it is
func containsRationale(comment string) bool {
	// Expected failure: this helper function does not exist yet
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
