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

const (
	promptPhaseBuild  = "build"
	promptPhaseReview = "review"

	// Phase annotation delimiters in RULES.md section headers.
	phaseAnnotationPrefix = "<!-- phases:"
	phaseAnnotationSuffix = "-->"
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

	// Validation history
	RecentValidationFailures []string // Summaries of recent validation failures from current run

	// Scoped test command for build phase self-checks (e.g. "go test ./internal/runner/... ./internal/config/...")
	// When non-empty, templates should use this instead of the generic "./..." form.
	ScopedTestCommand string

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
	if c.RecentValidationFailures == nil {
		c.RecentValidationFailures = []string{}
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
	ATDDActive bool // Whether ATDD methodology is active for this bead
}

// ScopeContext holds data for scope estimation prompt template
type ScopeContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
}

// PrecheckContext holds data for precheck prompt template
type PrecheckContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
}

// SpecAcceptanceContext holds data for spec acceptance prompt template
type SpecAcceptanceContext struct {
	Spec  string
	Rules string
}

// SpecGateContext holds data for spec gate prompt template
type SpecGateContext struct {
	SpecCriteria       string
	FailureOutput      string
	TestOutput         string // Output from running acceptance tests
	CumulativeDiff     string // Cumulative git diff for the spec
	AcceptanceCriteria string // Acceptance criteria from the spec file
}

// TestFixContext holds data for test-fix prompt template
type TestFixContext struct {
	ClaudeMD          string
	Rules             string
	TestCommand       string
	TestFailureOutput string
}

// TDDRedContext holds data for TDD red-phase prompt template.
type TDDRedContext struct {
	BeadID            string
	BeadTitle         string
	SpecExcerpt       string
	TestFileContents  map[string]string
	APISurface        string
	CycleSummary      string
	Rules             string
	WorkDir           string
	ScopedTestCommand string
	IsRetry           bool
	FailureContext    string
	PrevFailure       string
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
	templatesDir           string
	specsDir               string
	claudeMDPath           string
	rulesPath              string
	gromitDir              string
	learningsFile          *learnings.File
	maxLearningChars       int // Character budget for confirmed learnings; 0 means no cap
	budgetMaxChars         int // Total prompt budget in chars; 0 means no budget shaping
	budgetLearningCapChars int // Learning cap used during budget shaping

	// Cache fields - files are immutable during a run, so cache after first load
	claudeMDCache *string                       // Cached CLAUDE.md content
	rulesCache    *string                       // Cached RULES.md content
	specCache     map[string]string             // Cached spec files by name
	templateCache map[string]*template.Template // Cached parsed templates by name
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
		gromitDir:     gromitDir,
		learningsFile: lf,
		specCache:     make(map[string]string),
	}, nil
}

// GetLearningsFile returns the learnings file for external use
func (r *Renderer) GetLearningsFile() *learnings.File {
	if r == nil {
		return nil
	}
	return r.learningsFile
}

// GetSpecsDir returns the specs directory path
func (r *Renderer) GetSpecsDir() string {
	if r == nil {
		return ""
	}
	return r.specsDir
}

// GetGromitDir returns the gromit directory path
func (r *Renderer) GetGromitDir() string {
	if r == nil {
		return ""
	}
	return r.gromitDir
}

// SetMaxLearningChars sets the character budget for confirmed learnings.
// Zero means no cap (backward compatible).
func (r *Renderer) SetMaxLearningChars(maxChars int) {
	if r == nil {
		return
	}
	r.maxLearningChars = maxChars
}

// SetBudgetConfig sets the prompt budget shaping configuration.
// When maxChars > 0, qualifying render methods will call ShapeContextForBudget
// before rendering to trim context that exceeds the budget.
func (r *Renderer) SetBudgetConfig(maxChars, learningCapChars int) {
	if r == nil {
		return
	}
	r.budgetMaxChars = maxChars
	r.budgetLearningCapChars = learningCapChars
}

// RenderBuild renders the build prompt for a bead
func (r *Renderer) RenderBuild(ctx *Context) (string, error) {
	ctx = r.shapeBuildContext(ctx, promptPhaseBuild)
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

// RenderPrecheck renders the precheck prompt
func (r *Renderer) RenderPrecheck(ctx *PrecheckContext) (string, error) {
	return r.render("PROMPT_precheck.md", ctx)
}

// RenderSpecAcceptance renders the spec acceptance prompt
func (r *Renderer) RenderSpecAcceptance(ctx *SpecAcceptanceContext) (string, error) {
	return r.render("PROMPT_spec_acceptance.md", ctx)
}

// RenderSpecGate renders the spec gate prompt
func (r *Renderer) RenderSpecGate(ctx *SpecGateContext) (string, error) {
	return r.render("PROMPT_spec_gate.md", ctx)
}

// RenderReview renders the light review prompt
func (r *Renderer) RenderReview(ctx *ReviewContext) (string, error) {
	ctx = r.shapeReviewContext(ctx)
	return r.render("PROMPT_review.md", ctx)
}

// RenderThoroughReview renders the thorough review prompt
func (r *Renderer) RenderThoroughReview(ctx *ThoroughReviewContext) (string, error) {
	ctx = r.shapeThoroughReviewContext(ctx)
	return r.render("PROMPT_thorough_review.md", ctx)
}

// RenderAcceptanceTests renders the acceptance tests prompt for ATDD workflow
func (r *Renderer) RenderAcceptanceTests(ctx *Context) (string, error) {
	ctx = r.shapeBuildContext(ctx, promptPhaseBuild)
	return r.render("PROMPT_acceptance_tests.md", ctx)
}

// RenderATDDBuild renders the ATDD-aware build prompt
func (r *Renderer) RenderATDDBuild(ctx *Context) (string, error) {
	ctx = r.shapeBuildContext(ctx, promptPhaseBuild)
	return r.render("PROMPT_atdd_build.md", ctx)
}

// RenderRefactor renders the refactor prompt for code quality improvements
func (r *Renderer) RenderRefactor(ctx *Context) (string, error) {
	ctx = r.shapeBuildContext(ctx, promptPhaseBuild)
	return r.render("PROMPT_refactor.md", ctx)
}

// RenderTDDBuild renders the TDD-aware build prompt
func (r *Renderer) RenderTDDBuild(ctx *Context) (string, error) {
	ctx = r.shapeBuildContext(ctx, promptPhaseBuild)
	return r.render("PROMPT_tdd_build.md", ctx)
}

// RenderTDDRed renders the TDD red-phase prompt.
func (r *Renderer) RenderTDDRed(ctx *TDDRedContext) (string, error) {
	return r.render("PROMPT_tdd_red.md", ctx)
}

// RenderTestFix renders the test-fix prompt for fixing implementation without changing tests
func (r *Renderer) RenderTestFix(ctx *TestFixContext) (string, error) {
	return r.render("PROMPT_test_fix.md", ctx)
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

	// Lazy initialize cache if needed (tests may create Renderer directly)
	if r.specCache == nil {
		r.specCache = make(map[string]string)
	}

	// Return cached content if already loaded
	if content, ok := r.specCache[name]; ok {
		return content, nil
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
	contentStr := string(content)
	r.specCache[name] = contentStr
	return contentStr, nil
}

// LoadClaudeMD loads the project's CLAUDE.md
func (r *Renderer) LoadClaudeMD() (string, error) {
	if r == nil {
		return "", fmt.Errorf("renderer is nil")
	}
	// Return cached content if already loaded
	if r.claudeMDCache != nil {
		return *r.claudeMDCache, nil
	}
	content, err := os.ReadFile(r.claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading CLAUDE.md: %w", err)
	}
	contentStr := string(content)
	r.claudeMDCache = &contentStr
	return contentStr, nil
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
		ctx.ConfirmedLearnings = r.learningsFile.GetConfirmedFiltered(learnings.FilterOptions{
			MaxChars: r.maxLearningChars,
		})
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
	// Return cached content if already loaded
	if r.rulesCache != nil {
		return *r.rulesCache, nil
	}
	content, err := os.ReadFile(r.rulesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading RULES.md: %w", err)
	}
	contentStr := string(content)
	r.rulesCache = &contentStr
	return contentStr, nil
}

// LoadRulesForPhase loads RULES.md and returns only sections matching the given phase.
// Sections with <!-- phases: build, review --> annotations are included only when the
// requested phase appears in their phase list. Sections without annotations are included
// in all phases. Phase annotation comments are stripped from the output.
func (r *Renderer) LoadRulesForPhase(phase string) (string, error) {
	content, err := r.LoadRules()
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", nil
	}

	return filterRulesByPhase(content, phase), nil
}

// filterRulesByPhase parses RULES.md content, filters ## sections by phase annotations,
// and strips annotation comments from the output.
func filterRulesByPhase(content, phase string) string {
	lines := strings.Split(content, "\n")

	type section struct {
		headerLine string   // The ## header line (with annotation)
		bodyLines  []string // Lines after the header until the next ## header
		phases     []string // Parsed phases from annotation (nil = all phases)
	}

	var preamble []string // Lines before the first ## header
	var sections []section
	var current *section

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			// Save previous section
			if current != nil {
				sections = append(sections, *current)
			}
			// Parse phase annotation from the header
			phases := parsePhaseAnnotation(line)
			current = &section{
				headerLine: line,
				phases:     phases,
			}
		} else if current != nil {
			current.bodyLines = append(current.bodyLines, line)
		} else {
			preamble = append(preamble, line)
		}
	}
	// Don't forget the last section
	if current != nil {
		sections = append(sections, *current)
	}

	// Build output: preamble + matching sections
	var out strings.Builder
	for _, line := range preamble {
		out.WriteString(line)
		out.WriteString("\n")
	}

	for _, s := range sections {
		if !sectionMatchesPhase(s.phases, phase) {
			continue
		}
		// Write header with annotation stripped
		out.WriteString(stripPhaseAnnotation(s.headerLine))
		out.WriteString("\n")
		for _, line := range s.bodyLines {
			out.WriteString(line)
			out.WriteString("\n")
		}
	}

	// Trim trailing newline to match input convention
	result := out.String()
	if strings.HasSuffix(result, "\n") {
		result = result[:len(result)-1]
	}
	return result
}

// parsePhaseAnnotation extracts phase names from a <!-- phases: build, review --> comment
// in a header line. Returns nil if no annotation is found (meaning all phases).
func parsePhaseAnnotation(headerLine string) []string {
	idx := strings.Index(headerLine, phaseAnnotationPrefix)
	if idx < 0 {
		return nil
	}
	endIdx := strings.Index(headerLine[idx:], phaseAnnotationSuffix)
	if endIdx < 0 {
		return nil
	}
	phaseStr := headerLine[idx+len(phaseAnnotationPrefix) : idx+endIdx]
	parts := strings.Split(phaseStr, ",")
	var phases []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			phases = append(phases, p)
		}
	}
	return phases
}

// sectionMatchesPhase returns true if the section should be included for the given phase.
// If phases is nil (no annotation), the section is included in all phases.
func sectionMatchesPhase(phases []string, phase string) bool {
	if phases == nil {
		return true
	}
	for _, p := range phases {
		if p == phase {
			return true
		}
	}
	return false
}

// stripPhaseAnnotation removes the <!-- phases: ... --> comment from a header line.
func stripPhaseAnnotation(headerLine string) string {
	idx := strings.Index(headerLine, phaseAnnotationPrefix)
	if idx < 0 {
		return headerLine
	}
	endIdx := strings.Index(headerLine[idx:], phaseAnnotationSuffix)
	if endIdx < 0 {
		return headerLine
	}
	// Strip the annotation and any trailing whitespace
	before := strings.TrimRight(headerLine[:idx], " ")
	after := headerLine[idx+endIdx+len(phaseAnnotationSuffix):]
	return before + after
}

// shapeBuildContext applies budget shaping to a Context before rendering.
// Returns the original context if budget is zero or context is under budget.
func (r *Renderer) shapeBuildContext(ctx *Context, phase string) *Context {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx
	}
	shaped, report := ShapeContextForBudget(ctx, r.budgetMaxChars, r.budgetLearningCapChars, phase)
	logBudgetTrim(report)
	return shaped
}

// shapeReviewContext applies budget shaping to a ReviewContext before rendering.
func (r *Renderer) shapeReviewContext(ctx *ReviewContext) *ReviewContext {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx
	}
	shaped, report := ShapeReviewContextForBudget(ctx, r.budgetMaxChars, promptPhaseReview)
	logBudgetTrim(report)
	return shaped
}

// shapeThoroughReviewContext applies budget shaping to a ThoroughReviewContext before rendering.
func (r *Renderer) shapeThoroughReviewContext(ctx *ThoroughReviewContext) *ThoroughReviewContext {
	if r == nil || r.budgetMaxChars <= 0 || ctx == nil {
		return ctx
	}
	shaped, report := ShapeThoroughReviewContextForBudget(ctx, r.budgetMaxChars, promptPhaseReview)
	logBudgetTrim(report)
	return shaped
}

func logBudgetTrim(report *ShapeReport) {
	if report == nil || len(report.TrimActions) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Prompt budget: %d -> %d chars (trimmed: %s)\n",
		report.BeforeChars, report.AfterChars, strings.Join(report.TrimActions, ", "))
}

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

// ParseScopeEstimate parses Claude's JSON scope estimate output into a ScopeEstimate struct
func ParseScopeEstimate(output string) (*ScopeEstimate, error) {
	if output == "" {
		return nil, fmt.Errorf("scope estimate output is empty")
	}

	var estimate ScopeEstimate
	if err := jsonutil.ExtractJSON(output, &estimate); err != nil {
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
