package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/ralph-runner/internal/bead"
	"github.com/danabrams/ralph-runner/internal/learnings"
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
	Rules       string // Content of RULES.md
	WorkDir     string // Working directory

	// Learnings
	ConfirmedLearnings []learnings.Learning
	RecentLearnings    []learnings.Learning

	// Iteration info
	Iteration      int
	Model          string
	IsRetry        bool
	PrevFailure    string // Output from previous failed attempt
	FailureContext string // Suggestion from failure analysis
}

// AnalyzeContext holds data for failure analysis prompt template
type AnalyzeContext struct {
	BeadID          string
	BeadTitle       string
	BeadDescription string
	FailureOutput   string
}

// Renderer loads and renders prompt templates
type Renderer struct {
	templatesDir  string
	specsDir      string
	claudeMDPath  string
	rulesPath     string
	learningsFile *learnings.File
}

// NewRenderer creates a new prompt renderer
func NewRenderer(templatesDir, specsDir, claudeMDPath, ralphDir string) *Renderer {
	lf := learnings.NewFile(ralphDir)
	lf.Load() // Ignore error - learnings are optional

	return &Renderer{
		templatesDir:  templatesDir,
		specsDir:      specsDir,
		claudeMDPath:  claudeMDPath,
		rulesPath:     filepath.Join(ralphDir, "RULES.md"),
		learningsFile: lf,
	}
}

// GetLearningsFile returns the learnings file for external use
func (r *Renderer) GetLearningsFile() *learnings.File {
	return r.learningsFile
}

// RenderBuild renders the build prompt for a bead
func (r *Renderer) RenderBuild(ctx *Context) (string, error) {
	return r.render("PROMPT_build.md", ctx)
}

// RenderAnalyze renders the failure analysis prompt
func (r *Renderer) RenderAnalyze(ctx *AnalyzeContext) (string, error) {
	return r.render("PROMPT_analyze.md", ctx)
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

	// Load RULES.md
	rules, err := r.LoadRules()
	if err != nil {
		return nil, err
	}
	ctx.Rules = rules

	// Load learnings
	if r.learningsFile != nil {
		ctx.ConfirmedLearnings = r.learningsFile.GetConfirmed()
		ctx.RecentLearnings = r.learningsFile.GetRecent(24) // Last 24 hours
	}

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

// LoadRules loads the RULES.md file
func (r *Renderer) LoadRules() (string, error) {
	content, err := os.ReadFile(r.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading RULES.md: %w", err)
	}
	return string(content), nil
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
