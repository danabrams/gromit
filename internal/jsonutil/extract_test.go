package jsonutil

import (
	"testing"
)

func TestExtractObject(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		target  any
		wantErr bool
	}{
		{
			name:    "pure json object",
			input:   `{"key": "value"}`,
			target:  &map[string]string{},
			wantErr: false,
		},
		{
			name:    "json with surrounding text",
			input:   `Some text before {"key": "value"} and after`,
			target:  &map[string]string{},
			wantErr: false,
		},
		{
			name:    "json with nested objects",
			input:   `{"outer": {"inner": "value"}}`,
			target:  &map[string]any{},
			wantErr: false,
		},
		{
			name:    "json with escaped quotes in string",
			input:   `{"text": "He said \"hello\""}`,
			target:  &map[string]string{},
			wantErr: false,
		},
		{
			name:    "json with nested braces in string",
			input:   `{"text": "Contains {braces}"}`,
			target:  &map[string]string{},
			wantErr: false,
		},
		{
			name:    "invalid json",
			input:   `{"key": "unclosed string`,
			target:  &map[string]string{},
			wantErr: true,
		},
		{
			name:    "no opening brace",
			input:   `no json here`,
			target:  &map[string]string{},
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			target:  &map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtractObject(tt.input, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractArray(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(any) bool
	}{
		{
			name:    "pure json array",
			input:   `[{"id": "1"}, {"id": "2"}]`,
			wantErr: false,
			check: func(v any) bool {
				arr, ok := v.(*[]map[string]string)
				return ok && len(*arr) == 2
			},
		},
		{
			name:    "array with surrounding text",
			input:   "Some text\n[1, 2, 3]\nand after",
			wantErr: false,
			check: func(v any) bool {
				arr, ok := v.(*[]int)
				return ok && len(*arr) == 3
			},
		},
		{
			name:    "nested arrays",
			input:   `[[1, 2], [3, 4]]`,
			wantErr: false,
			check: func(v any) bool {
				arr, ok := v.(*[][]int)
				return ok && len(*arr) == 2
			},
		},
		{
			name:    "array with strings containing brackets",
			input:   `["text[with]brackets", "normal"]`,
			wantErr: false,
			check: func(v any) bool {
				arr, ok := v.(*[]string)
				return ok && len(*arr) == 2 && (*arr)[0] == "text[with]brackets"
			},
		},
		{
			name:    "no opening bracket",
			input:   `no json here`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var target any
			switch tt.name {
			case "pure json array":
				target = &[]map[string]string{}
			case "array with surrounding text":
				target = &[]int{}
			case "nested arrays":
				target = &[][]int{}
			case "array with strings containing brackets":
				target = &[]string{}
			}

			err := ExtractArray(tt.input, target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractArray() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && tt.check != nil && !tt.check(target) {
				t.Errorf("ExtractArray() result validation failed for: %s", tt.name)
			}
		})
	}
}

func TestExtractCodeBlock(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "json code block with language",
			input:   "Here's the result:\n```json\n{\"key\": \"value\"}\n```\nDone.",
			wantErr: false,
		},
		{
			name:    "code block without language",
			input:   "Here's the result:\n```\n{\"key\": \"value\"}\n```\nDone.",
			wantErr: false,
		},
		{
			name:    "multiline json in code block",
			input:   "Result:\n```json\n{\n  \"key\": \"value\",\n  \"nested\": {\n    \"inner\": \"data\"\n  }\n}\n```\n",
			wantErr: false,
		},
		{
			name:    "no code block",
			input:   `{"key": "value"}`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := &map[string]any{}
			err := ExtractCodeBlock(tt.input, target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractCodeBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		target  any
		wantErr bool
	}{
		{
			name:    "direct object",
			input:   `{"key": "value"}`,
			target:  &map[string]any{},
			wantErr: false,
		},
		{
			name:    "code block",
			input:   "```json\n{\"key\": \"value\"}\n```",
			target:  &map[string]any{},
			wantErr: false,
		},
		{
			name:    "object in text",
			input:   `Before {"key": "value"} after`,
			target:  &map[string]any{},
			wantErr: false,
		},
		{
			name:    "direct array",
			input:   `[1, 2, 3]`,
			target:  &[]int{},
			wantErr: false,
		},
		{
			name:    "array in text",
			input:   `Before [1, 2, 3] after`,
			target:  &[]int{},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			target:  &map[string]any{},
			wantErr: true,
		},
		{
			name:    "no json at all",
			input:   `just some text with no json`,
			target:  &map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExtractJSON(tt.input, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExtractBracketedJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		open     byte
		close    byte
		expected string
	}{
		{
			name:     "simple object",
			input:    `{"key": "value"}`,
			open:     '{',
			close:    '}',
			expected: `{"key": "value"}`,
		},
		{
			name:     "nested objects",
			input:    `{"outer": {"inner": "value"}}`,
			open:     '{',
			close:    '}',
			expected: `{"outer": {"inner": "value"}}`,
		},
		{
			name:     "escaped quotes",
			input:    `{"text": "He said \"hello\""}`,
			open:     '{',
			close:    '}',
			expected: `{"text": "He said \"hello\""}`,
		},
		{
			name:     "braces in string",
			input:    `{"text": "Contains {nested}"}`,
			open:     '{',
			close:    '}',
			expected: `{"text": "Contains {nested}"}`,
		},
		{
			name:     "simple array",
			input:    `[1, 2, 3]`,
			open:     '[',
			close:    ']',
			expected: `[1, 2, 3]`,
		},
		{
			name:     "nested arrays",
			input:    `[[1, 2], [3, 4]]`,
			open:     '[',
			close:    ']',
			expected: `[[1, 2], [3, 4]]`,
		},
		{
			name:     "brackets in string",
			input:    `["text[with]brackets"]`,
			open:     '[',
			close:    ']',
			expected: `["text[with]brackets"]`,
		},
		{
			name:     "incomplete structure",
			input:    `{"key": "unclosed`,
			open:     '{',
			close:    '}',
			expected: "",
		},
		{
			name:     "doesn't start with open bracket",
			input:    `not a bracket`,
			open:     '{',
			close:    '}',
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractBracketedJSON(tt.input, tt.open, tt.close)
			if result != tt.expected {
				t.Errorf("extractBracketedJSON() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRealWorldExamples(t *testing.T) {
	t.Run("analyzer output with surrounding text", func(t *testing.T) {
		output := `Here is my analysis:

{
  "category": "syntax",
  "recoverable": true,
  "root_cause": "Missing import statement",
  "suggestion": "Add the fmt package import"
}

Hope this helps.`

		type Analysis struct {
			Category    string `json:"category"`
			Recoverable bool   `json:"recoverable"`
			RootCause   string `json:"root_cause"`
			Suggestion  string `json:"suggestion"`
		}

		var result Analysis
		err := ExtractObject(output, &result)
		if err != nil {
			t.Fatalf("ExtractObject() error = %v", err)
		}

		if result.Category != "syntax" || !result.Recoverable {
			t.Errorf("ExtractObject() = %+v, want syntax and recoverable=true", result)
		}
	})

	t.Run("proposals in code block", func(t *testing.T) {
		output := "Here are the proposals:\n\n```json\n{\n  \"consolidations\": [],\n  \"promotions\": [\n    {\n      \"learning_hash\": \"abc123\",\n      \"proposed_rule\": \"Always check for nil\",\n      \"section\": \"Safety\",\n      \"rationale\": \"Prevents panics\"\n    }\n  ],\n  \"archives\": [],\n  \"rule_changes\": []\n}\n```\n\nThese are good ideas."

		type PromotionProposal struct {
			LearningHash string `json:"learning_hash"`
			ProposedRule string `json:"proposed_rule"`
			Section      string `json:"section"`
			Rationale    string `json:"rationale"`
		}

		type Proposals struct {
			Consolidations []any               `json:"consolidations"`
			Promotions     []PromotionProposal `json:"promotions"`
			Archives       []any               `json:"archives"`
			RuleChanges    []any               `json:"rule_changes"`
		}

		var result Proposals
		err := ExtractCodeBlock(output, &result)
		if err != nil {
			t.Fatalf("ExtractCodeBlock() error = %v", err)
		}

		if len(result.Promotions) != 1 || result.Promotions[0].LearningHash != "abc123" {
			t.Errorf("ExtractCodeBlock() = %+v, want 1 promotion with hash abc123", result)
		}
	})

	t.Run("decompose output with array and text", func(t *testing.T) {
		output := `Here are the subtasks:

[
  {"title": "Setup database", "description": "Create schema", "depends_on": null, "acceptance_criteria": ["schema exists"]},
  {"title": "Create API", "description": "REST endpoints", "depends_on": 1, "acceptance_criteria": ["endpoints respond"]}
]

That's all for now.`

		type SubTask struct {
			Title              string   `json:"title"`
			Description        string   `json:"description"`
			DependsOn          *int     `json:"depends_on"`
			AcceptanceCriteria []string `json:"acceptance_criteria"`
		}

		var result []SubTask
		err := ExtractArray(output, &result)
		if err != nil {
			t.Fatalf("ExtractArray() error = %v", err)
		}

		if len(result) != 2 || result[0].Title != "Setup database" {
			t.Errorf("ExtractArray() = %+v, want 2 subtasks", result)
		}
	})
}

func BenchmarkExtractObject(b *testing.B) {
	input := `Some text before {"key": "value", "nested": {"inner": "data"}} and after`
	target := &map[string]any{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractObject(input, target)
	}
}

func BenchmarkExtractArray(b *testing.B) {
	input := `Before [1, 2, 3, 4, 5] after`
	target := &[]int{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractArray(input, target)
	}
}

func BenchmarkExtractCodeBlock(b *testing.B) {
	input := "```json\n{\"key\": \"value\"}\n```"
	target := &map[string]string{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractCodeBlock(input, target)
	}
}
