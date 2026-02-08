package agent

import (
	"strings"
	"testing"
)

// TestPickWithSingleAgent verifies Pick returns immediately when only one agent is available
func TestPickWithSingleAgent(t *testing.T) {
	agents := []string{"claude"}
	defaultAgent := "claude"

	// No input needed - should return without prompting
	r := strings.NewReader("")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() with single agent error = %v, want nil", err)
	}

	if result != "claude" {
		t.Errorf("Pick() = %q, want %q", result, "claude")
	}

	// Verify no prompt was displayed (nothing written to w)
	if w.String() != "" {
		t.Errorf("Pick() wrote output %q, want empty (no prompt for single agent)", w.String())
	}
}

// TestPickWithEmptyInput verifies Pick returns default agent when input is empty
func TestPickWithEmptyInput(t *testing.T) {
	agents := []string{"claude", "codex", "gemini"}
	defaultAgent := "codex"

	// Send just a newline (empty input)
	r := strings.NewReader("\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() with empty input error = %v, want nil", err)
	}

	if result != defaultAgent {
		t.Errorf("Pick() with empty input = %q, want default %q", result, defaultAgent)
	}

	// Verify prompt was displayed
	output := w.String()
	if !strings.Contains(output, "codex (default)") {
		t.Error("Pick() output should mark default agent")
	}
}

// TestPickWithValidChoice verifies Pick returns selected agent for valid numeric input
func TestPickWithValidChoice(t *testing.T) {
	tests := []struct {
		name         string
		agents       []string
		defaultAgent string
		input        string
		wantAgent    string
	}{
		{
			name:         "choose first agent",
			agents:       []string{"claude", "codex", "gemini"},
			defaultAgent: "claude",
			input:        "1\n",
			wantAgent:    "claude",
		},
		{
			name:         "choose second agent",
			agents:       []string{"claude", "codex", "gemini"},
			defaultAgent: "claude",
			input:        "2\n",
			wantAgent:    "codex",
		},
		{
			name:         "choose third agent",
			agents:       []string{"claude", "codex", "gemini"},
			defaultAgent: "claude",
			input:        "3\n",
			wantAgent:    "gemini",
		},
		{
			name:         "choose last agent in longer list",
			agents:       []string{"claude", "codex", "gemini", "custom1", "custom2"},
			defaultAgent: "claude",
			input:        "5\n",
			wantAgent:    "custom2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			w := &strings.Builder{}

			result, err := Pick(tt.agents, tt.defaultAgent, r, w)
			if err != nil {
				t.Fatalf("Pick() error = %v, want nil", err)
			}

			if result != tt.wantAgent {
				t.Errorf("Pick() = %q, want %q", result, tt.wantAgent)
			}
		})
	}
}

// TestPickRendersNumberedList verifies Pick displays agents in numbered list format
func TestPickRendersNumberedList(t *testing.T) {
	agents := []string{"claude", "codex", "gemini"}
	defaultAgent := "codex"

	r := strings.NewReader("1\n")
	w := &strings.Builder{}

	_, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}

	output := w.String()

	// Verify numbered list format
	if !strings.Contains(output, "1. claude") {
		t.Error("Pick() output should contain '1. claude'")
	}
	if !strings.Contains(output, "2. codex") {
		t.Error("Pick() output should contain '2. codex'")
	}
	if !strings.Contains(output, "3. gemini") {
		t.Error("Pick() output should contain '3. gemini'")
	}
}

// TestPickMarksDefaultAgent verifies Pick marks the default agent in the list
func TestPickMarksDefaultAgent(t *testing.T) {
	tests := []struct {
		name         string
		agents       []string
		defaultAgent string
		wantMarked   string
	}{
		{
			name:         "first agent is default",
			agents:       []string{"claude", "codex", "gemini"},
			defaultAgent: "claude",
			wantMarked:   "claude (default)",
		},
		{
			name:         "middle agent is default",
			agents:       []string{"claude", "codex", "gemini"},
			defaultAgent: "codex",
			wantMarked:   "codex (default)",
		},
		{
			name:         "last agent is default",
			agents:       []string{"claude", "codex", "gemini"},
			defaultAgent: "gemini",
			wantMarked:   "gemini (default)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader("1\n")
			w := &strings.Builder{}

			_, err := Pick(tt.agents, tt.defaultAgent, r, w)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}

			output := w.String()
			if !strings.Contains(output, tt.wantMarked) {
				t.Errorf("Pick() output should contain %q, got:\n%s", tt.wantMarked, output)
			}
		})
	}
}

// TestPickInvalidInputRePrompts verifies Pick re-prompts on invalid input
func TestPickInvalidInputRePrompts(t *testing.T) {
	tests := []struct {
		name      string
		agents    []string
		input     string
		wantAgent string
	}{
		{
			name:      "invalid then valid - non-numeric",
			agents:    []string{"claude", "codex", "gemini"},
			input:     "abc\n2\n",
			wantAgent: "codex",
		},
		{
			name:      "invalid then valid - out of range (too high)",
			agents:    []string{"claude", "codex", "gemini"},
			input:     "99\n1\n",
			wantAgent: "claude",
		},
		{
			name:      "invalid then valid - zero",
			agents:    []string{"claude", "codex", "gemini"},
			input:     "0\n3\n",
			wantAgent: "gemini",
		},
		{
			name:      "invalid then valid - negative",
			agents:    []string{"claude", "codex", "gemini"},
			input:     "-1\n2\n",
			wantAgent: "codex",
		},
		{
			name:      "multiple invalid then valid",
			agents:    []string{"claude", "codex", "gemini"},
			input:     "abc\n99\n0\n2\n",
			wantAgent: "codex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			w := &strings.Builder{}

			result, err := Pick(tt.agents, "claude", r, w)
			if err != nil {
				t.Fatalf("Pick() error = %v, want nil (should re-prompt, not error)", err)
			}

			if result != tt.wantAgent {
				t.Errorf("Pick() = %q, want %q", result, tt.wantAgent)
			}

			// Verify re-prompt occurred (output should contain multiple Choice prompts)
			output := w.String()
			choiceCount := strings.Count(output, "Choice [1-")
			if choiceCount < 2 {
				t.Errorf("Pick() should re-prompt after invalid input, found %d prompts, want >= 2", choiceCount)
			}
		})
	}
}

// TestPickInvalidInputRePromptsWithEmptyInput verifies re-prompt after empty input when no default
func TestPickInvalidInputRePromptsAfterEmptyWhenNoDefault(t *testing.T) {
	agents := []string{"claude", "codex", "gemini"}
	defaultAgent := "" // No default agent

	// First input is empty (should re-prompt), second is valid
	r := strings.NewReader("\n2\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v, want nil", err)
	}

	if result != "codex" {
		t.Errorf("Pick() = %q, want %q", result, "codex")
	}

	// Verify re-prompt occurred
	output := w.String()
	choiceCount := strings.Count(output, "Choice [1-")
	if choiceCount < 2 {
		t.Errorf("Pick() should re-prompt after empty input when no default, found %d prompts, want >= 2", choiceCount)
	}
}

// TestPickWithWhitespaceOnlyInput verifies whitespace-only input is treated as empty
func TestPickWithWhitespaceOnlyInput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultAgent string
		wantAgent    string
	}{
		{
			name:         "spaces only - returns default",
			input:        "   \n",
			defaultAgent: "codex",
			wantAgent:    "codex",
		},
		{
			name:         "tabs only - returns default",
			input:        "\t\t\n",
			defaultAgent: "gemini",
			wantAgent:    "gemini",
		},
		{
			name:         "mixed whitespace - returns default",
			input:        " \t \n",
			defaultAgent: "claude",
			wantAgent:    "claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents := []string{"claude", "codex", "gemini"}
			r := strings.NewReader(tt.input)
			w := &strings.Builder{}

			result, err := Pick(agents, tt.defaultAgent, r, w)
			if err != nil {
				t.Fatalf("Pick() error = %v, want nil", err)
			}

			if result != tt.wantAgent {
				t.Errorf("Pick() with whitespace input = %q, want default %q", result, tt.wantAgent)
			}
		})
	}
}

// TestPickErrorOnEmptyAgentsList verifies Pick returns error when agents list is empty
func TestPickErrorOnEmptyAgentsList(t *testing.T) {
	agents := []string{}
	defaultAgent := "claude"

	r := strings.NewReader("1\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err == nil {
		t.Error("Pick() with empty agents list: error = nil, want error")
	}

	if result != "" {
		t.Errorf("Pick() with error should return empty string, got %q", result)
	}
}

// TestPickErrorOnNilAgentsList verifies Pick returns error when agents is nil
func TestPickErrorOnNilAgentsList(t *testing.T) {
	var agents []string = nil
	defaultAgent := "claude"

	r := strings.NewReader("1\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err == nil {
		t.Error("Pick() with nil agents list: error = nil, want error")
	}

	if result != "" {
		t.Errorf("Pick() with error should return empty string, got %q", result)
	}
}

// TestPickReadsFromCustomReader verifies Pick uses provided io.Reader
func TestPickReadsFromCustomReader(t *testing.T) {
	agents := []string{"claude", "codex", "gemini"}
	defaultAgent := "claude"

	// Custom reader that provides choice "3"
	input := "3\n"
	r := strings.NewReader(input)
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}

	if result != "gemini" {
		t.Errorf("Pick() = %q, want %q (should read from provided reader)", result, "gemini")
	}
}

// TestPickWritesToCustomWriter verifies Pick uses provided io.Writer
func TestPickWritesToCustomWriter(t *testing.T) {
	agents := []string{"claude", "codex"}
	defaultAgent := "claude"

	r := strings.NewReader("1\n")
	w := &strings.Builder{}

	_, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}

	output := w.String()
	if output == "" {
		t.Error("Pick() should write prompt to provided writer, got empty output")
	}

	// Verify output was written to our custom writer
	if !strings.Contains(output, "1. claude") {
		t.Error("Pick() should write numbered list to provided writer")
	}
}

// TestPickPreservesAgentOrder verifies Pick displays agents in the provided order
func TestPickPreservesAgentOrder(t *testing.T) {
	tests := []struct {
		name   string
		agents []string
	}{
		{
			name:   "alphabetical order",
			agents: []string{"alice", "bob", "charlie"},
		},
		{
			name:   "reverse alphabetical order",
			agents: []string{"zebra", "yankee", "xray"},
		},
		{
			name:   "mixed case order",
			agents: []string{"Claude", "codex", "GEMINI", "aider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader("1\n")
			w := &strings.Builder{}

			_, err := Pick(tt.agents, tt.agents[0], r, w)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}

			output := w.String()

			// Find positions of each agent in output
			lastPos := -1
			for i, agent := range tt.agents {
				pos := strings.Index(output, agent)
				if pos == -1 {
					t.Errorf("Agent %q not found in output", agent)
					continue
				}

				// Each agent should appear after the previous one
				if pos <= lastPos {
					t.Errorf("Agent %d (%q) appears at position %d, should be after position %d (order not preserved)",
						i, agent, pos, lastPos)
				}
				lastPos = pos
			}
		})
	}
}

// TestPickChoiceRangeDisplay verifies Pick shows correct choice range in prompt
func TestPickChoiceRangeDisplay(t *testing.T) {
	tests := []struct {
		name       string
		agents     []string
		wantPrompt string
	}{
		{
			name:       "two agents",
			agents:     []string{"claude", "codex"},
			wantPrompt: "Choice [1-2]:",
		},
		{
			name:       "three agents",
			agents:     []string{"claude", "codex", "gemini"},
			wantPrompt: "Choice [1-3]:",
		},
		{
			name:       "five agents",
			agents:     []string{"a", "b", "c", "d", "e"},
			wantPrompt: "Choice [1-5]:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader("1\n")
			w := &strings.Builder{}

			_, err := Pick(tt.agents, tt.agents[0], r, w)
			if err != nil {
				t.Fatalf("Pick() error = %v", err)
			}

			output := w.String()
			if !strings.Contains(output, tt.wantPrompt) {
				t.Errorf("Pick() output should contain %q, got:\n%s", tt.wantPrompt, output)
			}
		})
	}
}

// TestPickWithNonDefaultChoice verifies Pick returns non-default when explicitly chosen
func TestPickWithNonDefaultChoice(t *testing.T) {
	agents := []string{"claude", "codex", "gemini"}
	defaultAgent := "claude"

	// Choose codex (not the default)
	r := strings.NewReader("2\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}

	if result != "codex" {
		t.Errorf("Pick() = %q, want %q (should return chosen agent, not default)", result, "codex")
	}

	// Default should still be marked in output
	output := w.String()
	if !strings.Contains(output, "claude (default)") {
		t.Error("Pick() should mark default even when non-default is chosen")
	}
}

// TestPickDefaultNotInList verifies Pick handles default agent not in list
func TestPickDefaultNotInList(t *testing.T) {
	agents := []string{"codex", "gemini"}
	defaultAgent := "claude" // Not in the list

	r := strings.NewReader("1\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v", err)
	}

	if result != "codex" {
		t.Errorf("Pick() = %q, want %q", result, "codex")
	}

	// Should not mark any agent as default (or handle gracefully)
	output := w.String()
	if strings.Contains(output, "(default)") {
		t.Error("Pick() should not mark any agent as default when default is not in list")
	}
}

// TestPickEmptyInputWithDefaultNotInList verifies behavior when default is missing and input is empty
func TestPickEmptyInputWithDefaultNotInList(t *testing.T) {
	agents := []string{"codex", "gemini"}
	defaultAgent := "claude" // Not in the list

	// First input is empty (no valid default), second is valid choice
	r := strings.NewReader("\n1\n")
	w := &strings.Builder{}

	result, err := Pick(agents, defaultAgent, r, w)
	if err != nil {
		t.Fatalf("Pick() error = %v, want nil", err)
	}

	if result != "codex" {
		t.Errorf("Pick() = %q, want %q", result, "codex")
	}

	// Should re-prompt since default is not valid
	output := w.String()
	choiceCount := strings.Count(output, "Choice [1-")
	if choiceCount < 2 {
		t.Errorf("Pick() should re-prompt when default is not in list and input is empty, found %d prompts", choiceCount)
	}
}
