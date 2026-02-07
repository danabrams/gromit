package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestConfirmPrompt(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		// Lowercase responses
		{
			name:       "y with default yes",
			input:      "y\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "y with default no",
			input:      "y\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "n with default yes",
			input:      "n\n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "n with default no",
			input:      "n\n",
			defaultYes: false,
			want:       false,
		},
		// Uppercase responses
		{
			name:       "Y with default yes",
			input:      "Y\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "N with default no",
			input:      "N\n",
			defaultYes: false,
			want:       false,
		},
		// Full word responses
		{
			name:       "yes with default no",
			input:      "yes\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "no with default yes",
			input:      "no\n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "YES uppercase",
			input:      "YES\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "NO uppercase",
			input:      "NO\n",
			defaultYes: true,
			want:       false,
		},
		// Empty input (uses default)
		{
			name:       "empty input with default yes",
			input:      "\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "empty input with default no",
			input:      "\n",
			defaultYes: false,
			want:       false,
		},
		// Whitespace-padded input
		{
			name:       "y with leading space",
			input:      " y\n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "n with trailing space",
			input:      "n \n",
			defaultYes: true,
			want:       false,
		},
		{
			name:       "yes with surrounding whitespace",
			input:      "  yes  \n",
			defaultYes: false,
			want:       true,
		},
		{
			name:       "no with surrounding whitespace",
			input:      "  no  \n",
			defaultYes: true,
			want:       false,
		},
		// Invalid input (uses default)
		{
			name:       "invalid input with default yes",
			input:      "maybe\n",
			defaultYes: true,
			want:       true,
		},
		{
			name:       "invalid input with default no",
			input:      "maybe\n",
			defaultYes: false,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a bufio.Reader from the test input
			reader := bufio.NewReader(strings.NewReader(tt.input))

			// Call confirmPrompt
			got := confirmPrompt(reader, "Test prompt", tt.defaultYes)

			// Check result
			if got != tt.want {
				t.Errorf("confirmPrompt() = %v, want %v", got, tt.want)
			}
		})
	}
}
