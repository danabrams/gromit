package prompt

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"
)

func TestTemplatesWithNilBeadRender(t *testing.T) {
	t.Parallel()

	baseContext := &Context{
		Rules: "Project rules go here.",
		Spec:  "Spec content",
	}

	tests := []struct {
		name string
		file string
		ctx  interface{}
	}{
		{
			name: "build",
			file: "PROMPT_build.md",
			ctx:  baseContext,
		},
		{
			name: "refactor",
			file: "PROMPT_refactor.md",
			ctx:  baseContext,
		},
		{
			name: "tdd_build",
			file: "PROMPT_tdd_build.md",
			ctx:  baseContext,
		},
		{
			name: "atdd_build",
			file: "PROMPT_atdd_build.md",
			ctx:  baseContext,
		},
		{
			name: "acceptance_tests",
			file: "PROMPT_acceptance_tests.md",
			ctx:  baseContext,
		},
		{
			name: "scope",
			file: "PROMPT_scope.md",
			ctx:  &ScopeContext{},
		},
		{
			name: "precheck",
			file: "PROMPT_precheck.md",
			ctx:  &PrecheckContext{},
		},
		{
			name: "decompose",
			file: "PROMPT_decompose.md",
			ctx:  &DecomposeContext{},
		},
		{
			name: "review",
			file: "PROMPT_review.md",
			ctx:  &ReviewContext{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			path := filepath.Join("..", "..", ".gromit", "templates", tt.file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read template %s: %v", path, err)
			}

			tmpl, err := template.New(tt.file).
				Option("missingkey=error").
				Funcs(templateFuncs()).
				Parse(string(content))
			if err != nil {
				t.Fatalf("parse template %s: %v", tt.file, err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tt.ctx); err != nil {
				t.Fatalf("rendering %s: %v", tt.file, err)
			}
		})
	}
}
