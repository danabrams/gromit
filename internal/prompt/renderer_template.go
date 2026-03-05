package prompt

import (
	"bytes"
	"errors"
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

	tmpl, ok := r.templateCache[templateName]
	if !ok {
		path := filepath.Join(r.templatesDir, templateName)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("reading template %s: %w", templateName, err)
		}

		tmpl, err = r.parseAndCacheTemplate(templateName, string(content))
		if err != nil {
			return "", fmt.Errorf("parsing template %s: %w", templateName, err)
		}
	}

	return r.executeTemplate(templateName, tmpl, ctx)
}

func (r *Renderer) renderWithDefault(templateName, defaultContent string, ctx any) (string, error) {
	result, err := r.render(templateName, ctx)
	if err == nil {
		return result, nil
	}
	if defaultContent == "" {
		return "", err
	}
	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return r.renderFromString(templateName, defaultContent, ctx)
}

func (r *Renderer) renderFromString(templateName, content string, ctx any) (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	tmpl, ok := r.templateCache[templateName]
	if !ok {
		parsed, err := r.parseAndCacheTemplate(templateName, content)
		if err != nil {
			return "", fmt.Errorf("parsing template %s: %w", templateName, err)
		}
		tmpl = parsed
	}
	return r.executeTemplate(templateName, tmpl, ctx)
}

func (r *Renderer) parseAndCacheTemplate(templateName, content string) (*template.Template, error) {
	tmpl, err := template.New(templateName).Option("missingkey=error").Funcs(templateFuncs()).Parse(content)
	if err != nil {
		return nil, err
	}
	if r.templateCache == nil {
		r.templateCache = make(map[string]*template.Template)
	}
	r.templateCache[templateName] = tmpl
	return tmpl, nil
}

func (r *Renderer) executeTemplate(templateName string, tmpl *template.Template, ctx any) (string, error) {
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
		"defaultInt": func(value, fallback int) int {
			if value != 0 {
				return value
			}
			return fallback
		},
		"defaultString": func(value, fallback string) string {
			if value != "" {
				return value
			}
			return fallback
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
