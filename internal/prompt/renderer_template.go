package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

func (r *Renderer) render(templateName string, ctx any) (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}

	// Use cached template if available; otherwise load from disk and cache.
	// Templates are frozen at first access so mid-run file changes (e.g. a
	// bead modifying its own templates) cannot break subsequent iterations.
	tmpl, ok := r.templateCache[templateName]
	if !ok {
		path := filepath.Join(r.templatesDir, templateName)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading template %s: %w", templateName, err)
		}

		tmpl, err = template.New(templateName).Option("missingkey=zero").Funcs(templateFuncs()).Parse(string(content))
		if err != nil {
			return "", fmt.Errorf("parsing template %s: %w", templateName, err)
		}

		if r.templateCache == nil {
			r.templateCache = make(map[string]*template.Template)
		}
		r.templateCache[templateName] = tmpl
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing template %s: %w", templateName, err)
	}

	return buf.String(), nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"join":     strings.Join,
		"contains": strings.Contains,
		"hasLabel": func(labels []string, target string) bool {
			return bead.HasLabel(labels, target)
		},
		"indent": func(spaces int, s string) string {
			pad := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = pad + line
				}
			}
			return strings.Join(lines, "\n")
		},
		"formatLearnings": func(ls []learnings.Learning) string {
			if len(ls) == 0 {
				return "*None*"
			}
			var sb strings.Builder
			for _, l := range ls {
				sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", l.Category, l.Content))
			}
			return sb.String()
		},
	}
}
