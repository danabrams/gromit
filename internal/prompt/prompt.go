package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/ralph-runner/internal/bead"
)

// Context holds all data available to prompt templates
type Context struct {
	// Current task
	Bead        *bead.Bead
	ParentBead  *bead.Bead
	Spec        string // Content of spec file if referenced
	SpecName    string // Name of spec file

	// Project context
	ClaudeMD    string // Content of project's CLAUDE.md
	WorkDir     string // Working directory

	// Iteration info
	Iteration   int
	Model       string
	IsRetry     bool
	PrevFailure string // Output from previous failed attempt
}

// Renderer loads and renders prompt templates
type Renderer struct {
	templatesDir string
	specsDir     string
	claudeMDPath string
}

// NewRenderer creates a new prompt renderer
func NewRenderer(templatesDir, specsDir, claudeMDPath string) *Renderer {
	return &Renderer{
		templatesDir: templatesDir,
		specsDir:     specsDir,
		claudeMDPath: claudeMDPath,
	}
}

// RenderBuild renders the build prompt for a bead
func (r *Renderer) RenderBuild(ctx *Context) (string, error) {
	return r.render("PROMPT_build.md", ctx)
}

// RenderValidate renders the validation prompt
func (r *Renderer) RenderValidate(ctx *Context, commands []string) (string, error) {
	// Add commands to context for validation template
	type ValidateContext struct {
		*Context
		Commands []string
	}
	vctx := &ValidateContext{Context: ctx, Commands: commands}
	return r.render("PROMPT_validate.md", vctx)
}

// LoadSpec loads a spec file by name
func (r *Renderer) LoadSpec(name string) (string, error) {
	path := filepath.Join(r.specsDir, name+".md")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // Spec doesn't exist - not an error
		}
		return "", fmt.Errorf("reading spec %s: %w", name, err)
	}
	return string(content), nil
}

// LoadClaudeMD loads the project's CLAUDE.md
func (r *Renderer) LoadClaudeMD() (string, error) {
	content, err := os.ReadFile(r.claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading CLAUDE.md: %w", err)
	}
	return string(content), nil
}

// BuildContext builds a complete prompt context for a bead
func (r *Renderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*Context, error) {
	ctx := &Context{
		Bead:       b,
		ParentBead: parent,
		Iteration:  iteration,
		Model:      model,
	}

	// Load CLAUDE.md
	claudeMD, err := r.LoadClaudeMD()
	if err != nil {
		return nil, err
	}
	ctx.ClaudeMD = claudeMD

	// Get working directory
	ctx.WorkDir, _ = os.Getwd()

	// Find and load spec (check bead first, then parent)
	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" && parent != nil {
		specName = bead.FindSpecLabel(parent.Labels)
	}

	if specName != "" {
		spec, err := r.LoadSpec(specName)
		if err != nil {
			return nil, err
		}
		ctx.Spec = spec
		ctx.SpecName = specName
	}

	return ctx, nil
}

func (r *Renderer) render(templateName string, ctx any) (string, error) {
	path := filepath.Join(r.templatesDir, templateName)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading template %s: %w", templateName, err)
	}

	tmpl, err := template.New(templateName).Funcs(templateFuncs()).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parsing template %s: %w", templateName, err)
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
	}
}
