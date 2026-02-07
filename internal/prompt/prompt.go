package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/jsonutil"
	"github.com/danabrams/gromit/internal/learnings"
)

// Context holds all data available to prompt templates
type Context struct {
	// Current task
	Bead       *bead.Bead
	ParentBead *bead.Bead
	Spec       string // Content of spec file if referenced
	SpecName   string // Name of spec file

	// Project context
	ClaudeMD string // Content of project's CLAUDE.md
	Rules    string // Content of RULES.md
	WorkDir  string // Working directory

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

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (c *Context) normalizeNilFields() {
	if c == nil {
		return
	}
	if c.ConfirmedLearnings == nil {
		c.ConfirmedLearnings = []learnings.Learning{}
	}
	if c.RecentLearnings == nil {
		c.RecentLearnings = []learnings.Learning{}
	}
}

// AnalyzeContext holds data for failure analysis prompt template
type AnalyzeContext struct {
	BeadID          string
	BeadTitle       string
	BeadDescription string
	FailureOutput   string
}

// LearnContext holds data for success learning extraction prompt template
type LearnContext struct {
	BeadID          string
	BeadTitle       string
	BeadDescription string
	Summary         string // Brief summary of work done
}

// SuccessLearning represents the result of extracting a learning from success
type SuccessLearning struct {
	Learning *string `json:"learning"` // nil if no learning extracted
	Category string  `json:"category"`
}

// DecomposeContext holds data for task decomposition prompt template
type DecomposeContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
}

// ScopeContext holds data for scope estimation prompt template
type ScopeContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
}

// ScopeEstimate represents the result of scope estimation
type ScopeEstimate struct {
	Complexity                   string   `json:"complexity"`
	EstimatedIterations          int      `json:"estimated_iterations"`
	Rationale                    string   `json:"rationale"`
	CanCompleteInSingleIteration bool     `json:"can_complete_in_single_iteration"`
	Blockers                     []string `json:"blockers"`
}

// normalizeNilFields ensures nil slices are replaced with empty slices.
func (s *ScopeEstimate) normalizeNilFields() {
	if s == nil {
		return
	}
	if s.Blockers == nil {
		s.Blockers = []string{}
	}
}

// ReviewContext holds data for light review prompt template
type ReviewContext struct {
	Bead               *bead.Bead
	ParentBead         *bead.Bead
	Spec               string
	Diff               string
	ClaudeMD           string
	Rules              string
	Model              string
	ValidationCommands []string
}

// CompletedBeadSummary holds summary information about a completed bead for thorough reviews
type CompletedBeadSummary struct {
	ID          string
	Title       string
	Description string
}

// ThoroughReviewContext holds data for thorough review prompt template
type ThoroughReviewContext struct {
	Diff           string
	CompletedBeads []CompletedBeadSummary
	ClaudeMD       string
	Rules          string
	Model          string
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
func NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir string) (*Renderer, error) {
	lf, err := learnings.NewFile(gromitDir)
	if err != nil {
		return nil, err
	}
	lf.Load() // Ignore error - learnings are optional

	return &Renderer{
		templatesDir:  templatesDir,
		specsDir:      specsDir,
		claudeMDPath:  claudeMDPath,
		rulesPath:     filepath.Join(gromitDir, "RULES.md"),
		learningsFile: lf,
	}, nil
}

// GetLearningsFile returns the learnings file for external use
func (r *Renderer) GetLearningsFile() *learnings.File {
	if r == nil {
		return nil
	}
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

// RenderLearn renders the success learning extraction prompt
func (r *Renderer) RenderLearn(ctx *LearnContext) (string, error) {
	return r.render("PROMPT_learn.md", ctx)
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

// RenderDecompose renders the task decomposition prompt
func (r *Renderer) RenderDecompose(ctx *DecomposeContext) (string, error) {
	return r.render("PROMPT_decompose.md", ctx)
}

// RenderScope renders the scope estimation prompt
func (r *Renderer) RenderScope(ctx *ScopeContext) (string, error) {
	return r.render("PROMPT_scope.md", ctx)
}

// RenderReview renders the light review prompt
func (r *Renderer) RenderReview(ctx *ReviewContext) (string, error) {
	return r.render("PROMPT_review.md", ctx)
}

// RenderThoroughReview renders the thorough review prompt
func (r *Renderer) RenderThoroughReview(ctx *ThoroughReviewContext) (string, error) {
	return r.render("PROMPT_thorough_review.md", ctx)
}

// RenderAcceptanceTests renders the acceptance tests prompt for ATDD workflow
func (r *Renderer) RenderAcceptanceTests(ctx *Context) (string, error) {
	return r.render("PROMPT_acceptance_tests.md", ctx)
}

// RenderATDDBuild renders the ATDD-aware build prompt
func (r *Renderer) RenderATDDBuild(ctx *Context) (string, error) {
	return r.render("PROMPT_atdd_build.md", ctx)
}

// RenderRefactor renders the refactor prompt for code quality improvements
func (r *Renderer) RenderRefactor(ctx *Context) (string, error) {
	return r.render("PROMPT_refactor.md", ctx)
}

// RenderTDDBuild renders the TDD-aware build prompt
func (r *Renderer) RenderTDDBuild(ctx *Context) (string, error) {
	return r.render("PROMPT_tdd_build.md", ctx)
}

// ValidateSpecName checks that a spec name doesn't contain path traversal characters
func ValidateSpecName(name string) error {
	if name == "" {
		return fmt.Errorf("empty spec name")
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid spec name %q: contains path traversal characters", name)
	}
	return nil
}

// LoadSpec loads a spec file by name
func (r *Renderer) LoadSpec(name string) (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	if err := ValidateSpecName(name); err != nil {
		return "", err
	}

	path := filepath.Join(r.specsDir, name+".md")

	// Belt-and-suspenders: verify resolved path stays within specsDir
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving spec path: %w", err)
	}
	absSpecsDir, err := filepath.Abs(r.specsDir)
	if err != nil {
		return "", fmt.Errorf("resolving specs dir: %w", err)
	}
	if !strings.HasPrefix(absPath, absSpecsDir+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid spec name %q: resolves outside specs directory", name)
	}

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
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
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
	if r == nil {
		return nil, fmt.Errorf("renderer is nil")
	}
	if b == nil {
		return nil, fmt.Errorf("bead is nil")
	}
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

	ctx.normalizeNilFields()
	return ctx, nil
}

// LoadRules loads the RULES.md file
func (r *Renderer) LoadRules() (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
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
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
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

// ParseScopeEstimate parses Claude's JSON scope estimate output into a ScopeEstimate struct
func ParseScopeEstimate(output string) (*ScopeEstimate, error) {
	if output == "" {
		return nil, fmt.Errorf("scope estimate output is empty")
	}

	var estimate ScopeEstimate
	if err := jsonutil.ExtractObject(output, &estimate); err != nil {
		return nil, fmt.Errorf("parsing scope estimate JSON: %w", err)
	}

	estimate.normalizeNilFields()

	return &estimate, nil
}

// ParseSuccessLearning parses Claude's JSON success learning output into a SuccessLearning struct
func ParseSuccessLearning(output string) (*SuccessLearning, error) {
	if output == "" {
		return nil, fmt.Errorf("success learning output is empty")
	}

	var learning SuccessLearning
	if err := jsonutil.ExtractObject(output, &learning); err != nil {
		return nil, fmt.Errorf("parsing success learning JSON: %w", err)
	}

	// Validate category
	switch learning.Category {
	case "conventions", "gotchas", "patterns":
		// Valid
	default:
		learning.Category = "gotchas" // Default
	}

	return &learning, nil
}
