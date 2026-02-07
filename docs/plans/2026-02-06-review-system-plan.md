# Review System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a two-layer review system — light post-iteration reviews and thorough periodic reviews — that catches issues early and prevents quality debt from accumulating across iterations.

**Architecture:** New `internal/review/` package with `ReviewResult` struct and JSON parsing. Config gets `ReviewConfig` and `ThoroughReviewConfig` types. State gets review tracking fields. Runner integrates light review after validation. New `gromit review` CLI command handles interactive and non-interactive thorough reviews.

**Tech Stack:** Go, cobra CLI, Go text/template, bd CLI, Claude CLI, git diff

**Design doc:** `docs/plans/2026-02-06-review-system-design.md`

---

### Task 1: Add ReviewConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing test**

```go
func TestReviewConfigDefaults(t *testing.T) {
	yaml := `models:
  p0: opus
`
	cfg := loadFromString(t, yaml)

	if cfg.Review.Model != "sonnet" {
		t.Errorf("expected default review model 'sonnet', got %q", cfg.Review.Model)
	}
	if cfg.Review.MatchBuildModel != true {
		t.Errorf("expected match_build_model default true")
	}
	if cfg.Review.Timeout != 120 {
		t.Errorf("expected default review timeout 120, got %d", cfg.Review.Timeout)
	}
	if cfg.Review.Thorough.Model != "opus" {
		t.Errorf("expected default thorough model 'opus', got %q", cfg.Review.Thorough.Model)
	}
	if cfg.Review.Thorough.EveryNIterations != 5 {
		t.Errorf("expected default every_n_iterations 5, got %d", cfg.Review.Thorough.EveryNIterations)
	}
	if cfg.Review.Thorough.OnEpicComplete != true {
		t.Errorf("expected on_epic_complete default true")
	}
	if cfg.Review.Thorough.Timeout != 900 {
		t.Errorf("expected default thorough timeout 900, got %d", cfg.Review.Thorough.Timeout)
	}
}

func TestReviewConfigExplicit(t *testing.T) {
	yaml := `review:
  enabled: true
  model: opus
  match_build_model: false
  timeout: 200
  thorough:
    enabled: true
    every_n_iterations: 10
    on_epic_complete: false
    model: sonnet
    timeout: 600
`
	cfg := loadFromString(t, yaml)

	if cfg.Review.Enabled != true {
		t.Errorf("expected review enabled true")
	}
	if cfg.Review.Model != "opus" {
		t.Errorf("expected review model 'opus', got %q", cfg.Review.Model)
	}
	if cfg.Review.MatchBuildModel != false {
		t.Errorf("expected match_build_model false")
	}
	if cfg.Review.Timeout != 200 {
		t.Errorf("expected review timeout 200, got %d", cfg.Review.Timeout)
	}
	if cfg.Review.Thorough.EveryNIterations != 10 {
		t.Errorf("expected every_n_iterations 10, got %d", cfg.Review.Thorough.EveryNIterations)
	}
	if cfg.Review.Thorough.OnEpicComplete != false {
		t.Errorf("expected on_epic_complete false")
	}
}
```

Note: `loadFromString` is a test helper. If it doesn't exist already in config_test.go, create it:

```go
func loadFromString(t *testing.T, yamlStr string) *Config {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "gromit.yaml")
	if err := os.WriteFile(path, []byte(yamlStr), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestReviewConfig -v`
Expected: FAIL — `ReviewConfig` type doesn't exist yet

**Step 3: Write implementation**

Add to `config.go`:

```go
type ReviewConfig struct {
	Enabled         bool                `yaml:"enabled"`
	Model           string              `yaml:"model"`
	MatchBuildModel bool                `yaml:"match_build_model"`
	Timeout         int                 `yaml:"timeout"`
	Thorough        ThoroughReviewConfig `yaml:"thorough"`
}

type ThoroughReviewConfig struct {
	Enabled           bool   `yaml:"enabled"`
	EveryNIterations  int    `yaml:"every_n_iterations"`
	OnEpicComplete    bool   `yaml:"on_epic_complete"`
	Model             string `yaml:"model"`
	Timeout           int    `yaml:"timeout"`
}
```

Add `Review ReviewConfig` field to `Config` struct.

Add defaults in `setDefaults()`:
```go
if c.Review.Model == "" {
	c.Review.Model = "sonnet"
}
if !c.Review.MatchBuildModel && c.Review.Model == "sonnet" {
	c.Review.MatchBuildModel = true // default true
}
if c.Review.Timeout == 0 {
	c.Review.Timeout = 120
}
if c.Review.Thorough.Model == "" {
	c.Review.Thorough.Model = "opus"
}
if c.Review.Thorough.EveryNIterations == 0 {
	c.Review.Thorough.EveryNIterations = 5
}
if c.Review.Thorough.Timeout == 0 {
	c.Review.Thorough.Timeout = 900
}
```

Note: `MatchBuildModel` and `OnEpicComplete` default to false from Go zero values but we want true. Handle this with a pointer or a separate `defaultsApplied` pattern — or simpler: always set them in `setDefaults()` if the `Review` block isn't explicitly configured. The simplest approach: check if the entire Review section was unmarshaled vs zero-valued.

Actually, the cleanest Go pattern for "default true" booleans in YAML: use `*bool` pointer fields so nil = unset (apply default), false = explicitly set to false. Or just document that `review.enabled` must be explicitly set to `true` to enable, and `match_build_model` defaults to true. For `match_build_model` and `on_epic_complete`, use the same approach used elsewhere: set in `setDefaults()` only if the parent section appears unused.

Simplest: make `Review.Enabled` the gate. If `Review.Enabled` is false (default), all review features are off. When someone sets `Review.Enabled: true`, they get sensible defaults including `match_build_model: true` and `on_epic_complete: true`.

Use a dedicated `reviewDefaultsSet` bool or simply unconditionally set `MatchBuildModel = true` and `OnEpicComplete = true` in `setDefaults()`, and let explicit YAML `false` override via the YAML unmarshaler running after `setDefaults()`. Wait — `setDefaults()` runs AFTER unmarshaling. So: we need the pointer approach.

Use `*bool` for `MatchBuildModel` and `OnEpicComplete`:

```go
type ReviewConfig struct {
	Enabled         bool                 `yaml:"enabled"`
	Model           string               `yaml:"model"`
	MatchBuildModel *bool                `yaml:"match_build_model"`
	Timeout         int                  `yaml:"timeout"`
	Thorough        ThoroughReviewConfig `yaml:"thorough"`
}

type ThoroughReviewConfig struct {
	Enabled          bool   `yaml:"enabled"`
	EveryNIterations int    `yaml:"every_n_iterations"`
	OnEpicComplete   *bool  `yaml:"on_epic_complete"`
	Model            string `yaml:"model"`
	Timeout          int    `yaml:"timeout"`
}
```

In `setDefaults()`:
```go
if c.Review.MatchBuildModel == nil {
	t := true
	c.Review.MatchBuildModel = &t
}
if c.Review.Thorough.OnEpicComplete == nil {
	t := true
	c.Review.Thorough.OnEpicComplete = &t
}
```

Add helper methods:
```go
func (r ReviewConfig) ShouldMatchBuildModel() bool {
	if r.MatchBuildModel == nil {
		return true
	}
	return *r.MatchBuildModel
}

func (t ThoroughReviewConfig) ShouldRunOnEpicComplete() bool {
	if t.OnEpicComplete == nil {
		return true
	}
	return *t.OnEpicComplete
}
```

**Step 4: Run tests**

Run: `go test ./internal/config/ -run TestReviewConfig -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add ReviewConfig and ThoroughReviewConfig to config"
```

---

### Task 2: Add review state tracking to state.json

**Files:**
- Modify: `internal/state/state.go`
- Test: `internal/state/state_test.go`

**Step 1: Write the failing test**

```go
func TestReviewState(t *testing.T) {
	tmpDir := t.TempDir()
	sf := NewFile(tmpDir)

	// Initial state has no review info
	if err := sf.Load(); err != nil {
		t.Fatal(err)
	}
	if sf.LastReviewCommit() != "" {
		t.Errorf("expected empty last review commit, got %q", sf.LastReviewCommit())
	}
	if sf.LastReviewIteration() != 0 {
		t.Errorf("expected 0 last review iteration, got %d", sf.LastReviewIteration())
	}
	if sf.IterationsSinceReview() != 0 {
		t.Errorf("expected 0 iterations since review, got %d", sf.IterationsSinceReview())
	}

	// Record a review
	if err := sf.RecordReview("abc123", 5); err != nil {
		t.Fatal(err)
	}

	// Reload and check
	sf2 := NewFile(tmpDir)
	if err := sf2.Load(); err != nil {
		t.Fatal(err)
	}
	if sf2.LastReviewCommit() != "abc123" {
		t.Errorf("expected 'abc123', got %q", sf2.LastReviewCommit())
	}
	if sf2.LastReviewIteration() != 5 {
		t.Errorf("expected 5, got %d", sf2.LastReviewIteration())
	}
}

func TestIncrementIterationsSinceReview(t *testing.T) {
	tmpDir := t.TempDir()
	sf := NewFile(tmpDir)
	if err := sf.Load(); err != nil {
		t.Fatal(err)
	}

	sf.IncrementIterationsSinceReview()
	if sf.IterationsSinceReview() != 1 {
		t.Errorf("expected 1, got %d", sf.IterationsSinceReview())
	}

	sf.IncrementIterationsSinceReview()
	sf.IncrementIterationsSinceReview()
	if sf.IterationsSinceReview() != 3 {
		t.Errorf("expected 3, got %d", sf.IterationsSinceReview())
	}

	// RecordReview resets counter
	if err := sf.RecordReview("def456", 8); err != nil {
		t.Fatal(err)
	}
	if sf.IterationsSinceReview() != 0 {
		t.Errorf("expected 0 after RecordReview, got %d", sf.IterationsSinceReview())
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/state/ -run TestReviewState -v`
Expected: FAIL — methods don't exist

**Step 3: Write implementation**

Add fields to `State` struct:
```go
type State struct {
	LastRetro              time.Time `json:"last_retro,omitempty"`
	LastReviewCommit       string    `json:"last_review_commit,omitempty"`
	LastReviewIteration    int       `json:"last_review_iteration,omitempty"`
	IterationsSinceReview  int       `json:"iterations_since_review,omitempty"`
}
```

Add accessor methods to `File`:
```go
func (f *File) LastReviewCommit() string { ... }
func (f *File) LastReviewIteration() int { ... }
func (f *File) IterationsSinceReview() int { ... }
func (f *File) IncrementIterationsSinceReview() { ... }
func (f *File) RecordReview(commit string, iteration int) error { ... }
```

`RecordReview` sets commit, iteration, resets counter to 0, and calls `Save()`.

**Step 4: Run tests**

Run: `go test ./internal/state/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/state/state.go internal/state/state_test.go
git commit -m "feat: add review state tracking (commit, iteration, counter)"
```

---

### Task 3: Add ReviewLog type to logger

**Files:**
- Modify: `internal/logger/logger.go`
- Test: `internal/logger/logger_test.go`

**Step 1: Write the failing test**

```go
func TestLogReview(t *testing.T) {
	tmpDir := t.TempDir()
	l, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	review := &ReviewLog{
		Timestamp:      time.Now(),
		Type:           "review",
		ReviewType:     "light",
		Iteration:      5,
		BeadID:         "abc-123",
		Model:          "sonnet",
		Passed:         true,
		FixesApplied:   1,
		BeadsCreated:   2,
		BacklogCreated: 0,
		DurationMs:     25000,
	}
	if err := l.LogReview(review); err != nil {
		t.Fatal(err)
	}

	// Read back and verify
	data, err := os.ReadFile(l.FilePath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"review_type":"light"`) {
		t.Errorf("log should contain review_type field")
	}
	if !strings.Contains(string(data), `"fixes_applied":1`) {
		t.Errorf("log should contain fixes_applied field")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/logger/ -run TestLogReview -v`
Expected: FAIL — `ReviewLog` type doesn't exist

**Step 3: Write implementation**

Add `ReviewLog` struct and `LogReview` method:

```go
type ReviewLog struct {
	Timestamp      time.Time `json:"timestamp"`
	Type           string    `json:"type"`
	ReviewType     string    `json:"review_type"`
	Iteration      int       `json:"iteration"`
	BeadID         string    `json:"bead_id"`
	Model          string    `json:"model"`
	Passed         bool      `json:"passed"`
	FixesApplied   int       `json:"fixes_applied"`
	BeadsCreated   int       `json:"beads_created"`
	BacklogCreated int       `json:"backlog_created"`
	DurationMs     int64     `json:"duration_ms"`
}

func (l *Logger) LogReview(log *ReviewLog) error {
	if l == nil {
		return nil
	}
	if err := l.ensureFile(); err != nil {
		return err
	}
	return l.encoder.Encode(log)
}
```

**Step 4: Run tests**

Run: `go test ./internal/logger/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/logger/logger.go internal/logger/logger_test.go
git commit -m "feat: add ReviewLog type and LogReview method to logger"
```

---

### Task 4: Create internal/review package — ReviewResult and parsing

**Files:**
- Create: `internal/review/review.go`
- Create: `internal/review/review_test.go`

**Step 1: Write the failing test**

```go
package review

import (
	"testing"
)

func TestParseReviewResult(t *testing.T) {
	input := `{
		"passed": true,
		"fixes_applied": ["added missing error check"],
		"beads_to_create": [
			{"title": "Add input validation", "description": "...", "priority": 1, "labels": ["from-review"]}
		],
		"backlog_items": [
			{"title": "Redesign auth flow", "description": "...", "reason": "needs product owner"}
		],
		"summary": "Implementation matches spec"
	}`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Error("expected passed=true")
	}
	if len(result.FixesApplied) != 1 {
		t.Errorf("expected 1 fix, got %d", len(result.FixesApplied))
	}
	if len(result.BeadsToCreate) != 1 {
		t.Errorf("expected 1 bead, got %d", len(result.BeadsToCreate))
	}
	if result.BeadsToCreate[0].Title != "Add input validation" {
		t.Errorf("unexpected bead title: %q", result.BeadsToCreate[0].Title)
	}
	if len(result.BacklogItems) != 1 {
		t.Errorf("expected 1 backlog item, got %d", len(result.BacklogItems))
	}
	if result.Summary != "Implementation matches spec" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
}

func TestParseReviewResultWithSurroundingText(t *testing.T) {
	input := `Here is my review:

{
	"passed": false,
	"fixes_applied": [],
	"beads_to_create": [],
	"backlog_items": [],
	"summary": "Major issues found"
}

That concludes my review.`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Error("expected passed=false")
	}
	if result.Summary != "Major issues found" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
}

func TestParseReviewResultNilSlices(t *testing.T) {
	input := `{"passed": true, "summary": "looks good"}`

	result, err := ParseReviewResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.FixesApplied == nil {
		t.Error("FixesApplied should be normalized to empty slice, not nil")
	}
	if result.BeadsToCreate == nil {
		t.Error("BeadsToCreate should be normalized to empty slice, not nil")
	}
	if result.BacklogItems == nil {
		t.Error("BacklogItems should be normalized to empty slice, not nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/review/ -v`
Expected: FAIL — package doesn't exist

**Step 3: Write implementation**

Create `internal/review/review.go`:

```go
package review

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ReviewResult struct {
	Passed        bool           `json:"passed"`
	FixesApplied  []string       `json:"fixes_applied"`
	BeadsToCreate []BeadProposal `json:"beads_to_create"`
	BacklogItems  []BacklogItem  `json:"backlog_items"`
	Summary       string         `json:"summary"`
}

type BeadProposal struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels"`
}

type BacklogItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
}

func (r *ReviewResult) normalizeNilFields() {
	if r == nil {
		return
	}
	if r.FixesApplied == nil {
		r.FixesApplied = []string{}
	}
	if r.BeadsToCreate == nil {
		r.BeadsToCreate = []BeadProposal{}
	}
	if r.BacklogItems == nil {
		r.BacklogItems = []BacklogItem{}
	}
	for i := range r.BeadsToCreate {
		if r.BeadsToCreate[i].Labels == nil {
			r.BeadsToCreate[i].Labels = []string{}
		}
	}
}

func ParseReviewResult(output string) (*ReviewResult, error) {
	if output == "" {
		return nil, fmt.Errorf("review output is empty")
	}

	output = strings.TrimSpace(output)

	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON found in review output")
	}

	jsonStr := output[start : end+1]

	var result ReviewResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parsing review JSON: %w", err)
	}

	result.normalizeNilFields()
	return &result, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/review/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/review/review.go internal/review/review_test.go
git commit -m "feat: add review package with ReviewResult type and JSON parsing"
```

---

### Task 5: Add review prompt context and renderer methods

**Files:**
- Modify: `internal/prompt/prompt.go`
- Test: `internal/prompt/prompt_test.go`

**Step 1: Write the failing test**

```go
func TestRenderReview(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	// Write a simple review template
	tmpl := `Review for: {{.Bead.Title}}
Diff: {{.Diff}}
Model: {{.Model}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_review.md"), []byte(tmpl), 0644)

	r := NewRenderer(templatesDir, filepath.Join(tmpDir, "specs"), "", tmpDir)

	ctx := &ReviewContext{
		Bead: &bead.Bead{ID: "test-1", Title: "Test bead", Labels: []string{}, ExpectedOutputs: []string{}},
		Diff: "diff --git a/file.go\n+added line",
		Model: "sonnet",
	}

	result, err := r.RenderReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Test bead") {
		t.Error("expected bead title in output")
	}
	if !strings.Contains(result, "added line") {
		t.Error("expected diff in output")
	}
}

func TestRenderThoroughReview(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	tmpl := `Thorough review
Beads completed: {{len .CompletedBeads}}
Diff: {{.Diff}}`
	os.WriteFile(filepath.Join(templatesDir, "PROMPT_thorough_review.md"), []byte(tmpl), 0644)

	r := NewRenderer(templatesDir, filepath.Join(tmpDir, "specs"), "", tmpDir)

	ctx := &ThoroughReviewContext{
		Diff: "full diff here",
		CompletedBeads: []CompletedBeadSummary{
			{ID: "b-1", Title: "First task"},
			{ID: "b-2", Title: "Second task"},
		},
	}

	result, err := r.RenderThoroughReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Beads completed: 2") {
		t.Error("expected 2 completed beads")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/prompt/ -run TestRenderReview -v`
Expected: FAIL — types don't exist

**Step 3: Write implementation**

Add to `prompt.go`:

```go
type ReviewContext struct {
	Bead       *bead.Bead
	ParentBead *bead.Bead
	Spec       string
	Diff       string
	ClaudeMD   string
	Rules      string
	Model      string
	ValidationCommands []string
}

type CompletedBeadSummary struct {
	ID          string
	Title       string
	Description string
}

type ThoroughReviewContext struct {
	Diff           string
	CompletedBeads []CompletedBeadSummary
	ClaudeMD       string
	Rules          string
	Model          string
}

func (r *Renderer) RenderReview(ctx *ReviewContext) (string, error) {
	return r.render("PROMPT_review.md", ctx)
}

func (r *Renderer) RenderThoroughReview(ctx *ThoroughReviewContext) (string, error) {
	return r.render("PROMPT_thorough_review.md", ctx)
}
```

**Step 4: Run tests**

Run: `go test ./internal/prompt/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/prompt/prompt.go internal/prompt/prompt_test.go
git commit -m "feat: add ReviewContext, ThoroughReviewContext, and render methods"
```

---

### Task 6: Create PROMPT_review.md default template

**Files:**
- Modify: `cmd/gromit/init.go` (add default template constant and write it during init)

**Step 1: Write the template**

Add to `init.go` a `defaultReviewTemplate` constant with the full light review prompt template. The template should cover all review dimensions from the design doc, instruct Claude to return structured JSON, and use Go template syntax for context variables (`.Bead.Title`, `.Diff`, `.Rules`, etc.).

**Step 2: Add template writing to `runInit`**

Add alongside the other template writes:
```go
reviewPath := filepath.Join(cwd, ".gromit/templates/PROMPT_review.md")
if err := writeFileIfNotExists(reviewPath, defaultReviewTemplate, forceInit); err != nil {
	return err
}
```

**Step 3: Run tests**

Run: `go build ./cmd/gromit/`
Expected: Compiles without errors

**Step 4: Commit**

```bash
git add cmd/gromit/init.go
git commit -m "feat: add default PROMPT_review.md template to init"
```

---

### Task 7: Create PROMPT_thorough_review.md default template

**Files:**
- Modify: `cmd/gromit/init.go`

Same pattern as Task 6 but for the thorough review template. This template includes additional dimensions (architectural assessment, cross-cutting concerns, pattern detection) and takes `ThoroughReviewContext` variables.

**Step 1-4:** Same pattern as Task 6.

**Step 5: Commit**

```bash
git add cmd/gromit/init.go
git commit -m "feat: add default PROMPT_thorough_review.md template to init"
```

---

### Task 8: Add git diff helper to runner

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**Step 1: Write the failing test**

```go
func TestGetGitDiff(t *testing.T) {
	// This test requires a git repo, so skip in CI if needed
	// Use the project's own repo for testing
	head, err := getGitHead()
	if err != nil {
		t.Skip("not in a git repo")
	}

	// Get diff from HEAD to HEAD (should be empty for committed state)
	diff, err := getGitDiff(head)
	if err != nil {
		t.Fatal(err)
	}
	// diff may or may not be empty depending on working tree state
	// just verify it doesn't error
	_ = diff
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestGetGitDiff -v`
Expected: FAIL — `getGitDiff` doesn't exist

**Step 3: Write implementation**

```go
// getGitDiff returns the full diff from fromCommit to the current working tree
func getGitDiff(fromCommit string) (string, error) {
	cmd := exec.Command("git", "diff", fromCommit)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/runner/ -run TestGetGitDiff -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat: add getGitDiff helper for full diff output"
```

---

### Task 9: Implement light review in runner (core logic)

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

This is the core integration. Add a `runLightReview` method to `Runner` that:

1. Gets git diff from `startCommit` to HEAD
2. Builds a `ReviewContext` with bead, diff, spec, rules, CLAUDE.md
3. Selects model (sonnet, or opus if build used opus via `match_build_model`)
4. Renders `PROMPT_review.md`
5. Calls Claude with the rendered prompt
6. Parses the `ReviewResult` JSON from output
7. Returns the result

And a `applyReviewResult` method that:
1. Creates beads from `BeadsToCreate` (with `from-review` label)
2. Creates backlog beads from `BacklogItems` (with `from-review` + `backlog` labels, P2)
3. Returns count of fixes applied (for re-validation decision)

**Step 1: Write the failing test**

Test `applyReviewResult` with a mock bead client (or just test the decision logic for model selection and the parse flow — since `runLightReview` calls Claude, it's hard to unit test the full flow).

```go
func TestSelectReviewModel(t *testing.T) {
	tests := []struct {
		name           string
		buildModel     string
		matchBuild     bool
		configModel    string
		expectedModel  string
	}{
		{"default sonnet", "sonnet", true, "sonnet", "sonnet"},
		{"match opus build", "opus", true, "sonnet", "opus"},
		{"no match, use config", "opus", false, "sonnet", "sonnet"},
		{"haiku build, match enabled", "haiku", true, "sonnet", "sonnet"}, // only match opus
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchBool := tt.matchBuild
			cfg := &config.Config{
				Review: config.ReviewConfig{
					Model:           tt.configModel,
					MatchBuildModel: &matchBool,
				},
			}
			got := selectReviewModel(cfg, tt.buildModel)
			if got != tt.expectedModel {
				t.Errorf("expected %q, got %q", tt.expectedModel, got)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestSelectReviewModel -v`
Expected: FAIL

**Step 3: Write implementation**

```go
func selectReviewModel(cfg *config.Config, buildModel string) string {
	if cfg == nil {
		return "sonnet"
	}
	if cfg.Review.ShouldMatchBuildModel() && buildModel == "opus" {
		return "opus"
	}
	return cfg.Review.Model
}

func (r *Runner) runLightReview(ctx context.Context, b *bead.Bead, parent *bead.Bead, startCommit string, buildModel string, iteration int) (*review.ReviewResult, error) {
	// Get diff
	diff, err := getGitDiff(startCommit)
	if err != nil {
		return nil, fmt.Errorf("getting git diff for review: %w", err)
	}
	if strings.TrimSpace(diff) == "" {
		return nil, nil // No changes to review
	}

	// Build context
	reviewCtx := &prompt.ReviewContext{
		Bead:       b,
		ParentBead: parent,
		Diff:       diff,
		Model:      selectReviewModel(r.cfg, buildModel),
	}
	// Load CLAUDE.md and rules
	reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()
	reviewCtx.Rules, _ = r.renderer.LoadRules()
	// Load spec
	specName := bead.FindSpecLabel(b.Labels)
	if specName == "" && parent != nil {
		specName = bead.FindSpecLabel(parent.Labels)
	}
	if specName != "" {
		reviewCtx.Spec, _ = r.renderer.LoadSpec(specName)
	}

	// Render prompt
	reviewPrompt, err := r.renderer.RenderReview(reviewCtx)
	if err != nil {
		return nil, fmt.Errorf("rendering review prompt: %w", err)
	}

	// Select model
	model := selectReviewModel(r.cfg, buildModel)

	// Call Claude
	timeout := time.Duration(r.cfg.Review.Timeout) * time.Second
	reviewTimeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	claudeResult, err := r.claude.Run(reviewTimeoutCtx, reviewPrompt, model)
	if err != nil {
		return nil, fmt.Errorf("review invocation: %w", err)
	}
	if claudeResult == nil {
		return nil, fmt.Errorf("review returned nil result")
	}

	// Parse result
	result, err := review.ParseReviewResult(claudeResult.Output)
	if err != nil {
		return nil, fmt.Errorf("parsing review result: %w", err)
	}

	return result, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/runner/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat: add runLightReview method and review model selection"
```

---

### Task 10: Implement applyReviewResult (bead creation from review findings)

**Files:**
- Modify: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**Step 1: Write the failing test**

Test the `applyReviewResult` method's bead creation logic. Since it calls `bd create`, write a test that verifies the correct arguments would be passed (or test with a real bd if available).

For unit testing, test the label assembly logic:

```go
func TestBuildReviewBeadLabels(t *testing.T) {
	proposal := review.BeadProposal{
		Labels: []string{"security"},
	}
	labels := buildReviewBeadLabels(proposal.Labels)
	if !bead.HasLabel(labels, "from-review") {
		t.Error("missing from-review label")
	}
	if !bead.HasLabel(labels, "security") {
		t.Error("missing security label")
	}
}

func TestBuildBacklogLabels(t *testing.T) {
	labels := buildBacklogLabels()
	if !bead.HasLabel(labels, "from-review") {
		t.Error("missing from-review label")
	}
	if !bead.HasLabel(labels, "backlog") {
		t.Error("missing backlog label")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestBuildReview -v`
Expected: FAIL

**Step 3: Write implementation**

```go
func buildReviewBeadLabels(proposalLabels []string) []string {
	labels := []string{"from-review"}
	for _, l := range proposalLabels {
		if l != "from-review" { // avoid duplication
			labels = append(labels, l)
		}
	}
	return labels
}

func buildBacklogLabels() []string {
	return []string{"from-review", "backlog"}
}

func (r *Runner) applyReviewResult(result *review.ReviewResult) (beadsCreated int, backlogCreated int) {
	if result == nil || r.beads == nil {
		return 0, 0
	}

	for _, bp := range result.BeadsToCreate {
		labels := buildReviewBeadLabels(bp.Labels)
		_, err := r.beads.Create(bp.Title, bp.Priority, labels, nil)
		if err != nil {
			r.log("Warning: failed to create review bead: %v", err)
			continue
		}
		beadsCreated++
		r.log("Created review bead: %s (P%d)", bp.Title, bp.Priority)
	}

	for _, bi := range result.BacklogItems {
		labels := buildBacklogLabels()
		_, err := r.beads.Create(bi.Title, 2, labels, nil) // P2 for backlog
		if err != nil {
			r.log("Warning: failed to create backlog bead: %v", err)
			continue
		}
		backlogCreated++
		r.log("Created backlog bead: %s (reason: %s)", bi.Title, bi.Reason)
	}

	return beadsCreated, backlogCreated
}
```

**Step 4: Run tests**

Run: `go test ./internal/runner/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat: add applyReviewResult for creating beads from review findings"
```

---

### Task 11: Integrate light review into processBead

**Files:**
- Modify: `internal/runner/runner.go`

This is the integration step. In `processBead`, after validation passes (line ~754), add the review step:

1. If `r.cfg.Review.Enabled`, call `r.runLightReview()`
2. If review finds `fixes_applied`, re-run validation
3. If re-validation fails, treat as build failure
4. Call `applyReviewResult` to create beads/backlog
5. Log the review result via `r.writeReviewLog()`

**Step 1: Write the integration code**

After the validation block in `processBead` (around line 754, before `result.Success = true`):

```go
// Run light review if enabled
if r.cfg.Review.Enabled {
	r.log("Running post-iteration review with model: %s", selectReviewModel(r.cfg, model))

	reviewResult, err := r.runLightReview(ctx, b, parent, startCommit, model, iteration)
	if err != nil {
		r.log("Warning: review failed: %v", err)
		// Review failure is non-blocking — continue
	} else if reviewResult != nil {
		r.log("Review: %s", reviewResult.Summary)

		// If fixes were applied, re-validate
		if len(reviewResult.FixesApplied) > 0 {
			r.log("Review applied %d fixes, re-validating...", len(reviewResult.FixesApplied))

			if r.cfg.Validation.Enabled {
				valResult, err := r.claude.RunValidation(ctx, r.cfg.Validation.Commands, r.cfg.Models.Validation, promptCtx.WorkDir)
				if err != nil {
					result.Error = fmt.Errorf("review re-validation invocation: %w", err)
					return result
				}
				if valResult == nil || !claude.IsValidationPassed(valResult) {
					result.Error = fmt.Errorf("review fixes broke validation")
					result.Output += "\n\n=== REVIEW RE-VALIDATION FAILED ===\n"
					if valResult != nil {
						result.Output += valResult.Output
					}
					return result
				}
				r.log("Re-validation passed")
			}
		}

		// Create beads/backlog from review findings
		beadsCreated, backlogCreated := r.applyReviewResult(reviewResult)

		// Log review result
		r.writeReviewLog(iteration, b.ID, selectReviewModel(r.cfg, model), reviewResult, beadsCreated, backlogCreated)
	}
}
```

**Step 2: Add writeReviewLog method**

```go
func (r *Runner) writeReviewLog(iteration int, beadID string, model string, result *review.ReviewResult, beadsCreated, backlogCreated int) {
	if r == nil || r.logger == nil || result == nil {
		return
	}
	r.logger.LogReview(&logger.ReviewLog{
		Timestamp:      time.Now(),
		Type:           "review",
		ReviewType:     "light",
		Iteration:      iteration,
		BeadID:         beadID,
		Model:          model,
		Passed:         result.Passed,
		FixesApplied:   len(result.FixesApplied),
		BeadsCreated:   beadsCreated,
		BacklogCreated: backlogCreated,
	})
}
```

**Step 3: Verify compilation and tests**

Run: `go build ./... && go test ./internal/runner/ -v`
Expected: Compiles and existing tests pass

**Step 4: Commit**

```bash
git add internal/runner/runner.go
git commit -m "feat: integrate light review into processBead after validation"
```

---

### Task 12: Add thorough review trigger (every N iterations)

**Files:**
- Modify: `internal/runner/runner.go`

In the main `Run` loop, after a successful iteration (bead closed, synced), increment `iterations_since_review` in state and check if thorough review should run.

**Step 1: Write implementation**

Add state tracking to the `Run` method. Load state at the start, increment counter after each successful iteration, and trigger thorough review when counter reaches threshold.

```go
// At start of Run(), load state
sf := state.NewFile(r.gromitDir)
if err := sf.Load(); err != nil {
	r.log("Warning: could not load state: %v", err)
}

// After closing bead successfully (after bd sync), add:
sf.IncrementIterationsSinceReview()
if err := sf.Save(); err != nil {
	r.log("Warning: could not save state: %v", err)
}

// Check thorough review trigger
if r.cfg.Review.Thorough.Enabled && sf.IterationsSinceReview() >= r.cfg.Review.Thorough.EveryNIterations {
	r.log("\n=== Thorough Review (every %d iterations) ===", r.cfg.Review.Thorough.EveryNIterations)
	r.runThoroughReview(ctx, sf, iteration)
}
```

**Step 2: Implement runThoroughReview stub**

```go
func (r *Runner) runThoroughReview(ctx context.Context, sf *state.File, iteration int) {
	// Get diff since last review
	fromCommit := sf.LastReviewCommit()
	if fromCommit == "" {
		// No previous review — use a reasonable default (e.g., last 50 commits)
		r.log("No previous review commit found, skipping thorough review scope detection")
		return
	}

	diff, err := getGitDiff(fromCommit)
	if err != nil {
		r.log("Warning: could not get diff for thorough review: %v", err)
		return
	}
	if strings.TrimSpace(diff) == "" {
		r.log("No changes since last thorough review, skipping")
		return
	}

	// Build context
	reviewCtx := &prompt.ThoroughReviewContext{
		Diff:  diff,
		Model: r.cfg.Review.Thorough.Model,
	}
	reviewCtx.ClaudeMD, _ = r.renderer.LoadClaudeMD()
	reviewCtx.Rules, _ = r.renderer.LoadRules()

	// Render prompt
	reviewPrompt, err := r.renderer.RenderThoroughReview(reviewCtx)
	if err != nil {
		r.log("Warning: could not render thorough review prompt: %v", err)
		return
	}

	// Call Claude with opus
	timeout := time.Duration(r.cfg.Review.Thorough.Timeout) * time.Second
	reviewCtxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	model := r.cfg.Review.Thorough.Model
	r.log("Running thorough review with model: %s", model)
	claudeResult, err := r.claude.Run(reviewCtxTimeout, reviewPrompt, model)
	if err != nil {
		r.log("Warning: thorough review failed: %v", err)
		return
	}
	if claudeResult == nil {
		r.log("Warning: thorough review returned nil result")
		return
	}

	// Parse and apply
	result, err := review.ParseReviewResult(claudeResult.Output)
	if err != nil {
		r.log("Warning: could not parse thorough review result: %v", err)
		return
	}

	r.log("Thorough review: %s", result.Summary)
	beadsCreated, backlogCreated := r.applyReviewResult(result)

	// If fixes applied, re-validate
	if len(result.FixesApplied) > 0 && r.cfg.Validation.Enabled {
		r.log("Thorough review applied %d fixes, re-validating...", len(result.FixesApplied))
		workDir, _ := os.Getwd()
		valResult, err := r.claude.RunValidation(ctx, r.cfg.Validation.Commands, r.cfg.Models.Validation, workDir)
		if err != nil || valResult == nil || !claude.IsValidationPassed(valResult) {
			r.log("Warning: thorough review fixes broke validation")
		} else {
			r.log("Re-validation passed")
		}
	}

	// Log review
	if r.logger != nil {
		r.logger.LogReview(&logger.ReviewLog{
			Timestamp:      time.Now(),
			Type:           "review",
			ReviewType:     "thorough",
			Iteration:      iteration,
			Model:          model,
			Passed:         result.Passed,
			FixesApplied:   len(result.FixesApplied),
			BeadsCreated:   beadsCreated,
			BacklogCreated: backlogCreated,
		})
	}

	// Update state
	currentCommit, _ := getGitHead()
	if err := sf.RecordReview(currentCommit, iteration); err != nil {
		r.log("Warning: could not record review state: %v", err)
	}
}
```

**Step 3: Verify compilation and tests**

Run: `go build ./... && go test ./... -short`
Expected: All pass

**Step 4: Commit**

```bash
git add internal/runner/runner.go
git commit -m "feat: add thorough review trigger every N iterations"
```

---

### Task 13: Add epic completion trigger for thorough review

**Files:**
- Modify: `internal/runner/runner.go`
- Modify: `internal/bead/bead.go`

After closing a bead, check if it was the last open child of an epic. If so, trigger thorough review scoped to that epic.

**Step 1: Add HasOpenChildren method to bead client**

```go
// HasOpenChildren checks if an epic has any remaining open child tasks
func (c *Client) HasOpenChildren(parentID string) (bool, error) {
	// List all open beads and check if any have this parent
	beads, err := c.List()
	if err != nil {
		return false, err
	}
	for _, b := range beads {
		if b.Parent == parentID {
			return true, nil
		}
	}
	return false, nil
}
```

**Step 2: Add check in runner after bead close**

In `Run()`, after closing a bead, check if parent epic is now complete:

```go
// Check epic completion trigger
if b.Parent != "" && r.cfg.Review.Thorough.Enabled && r.cfg.Review.Thorough.ShouldRunOnEpicComplete() {
	hasChildren, err := r.beads.HasOpenChildren(b.Parent)
	if err != nil {
		r.log("Warning: could not check epic children: %v", err)
	} else if !hasChildren {
		r.log("\n=== Thorough Review (epic %s complete) ===", b.Parent)
		r.runThoroughReview(ctx, sf, iteration)
	}
}
```

**Step 3: Write tests for HasOpenChildren**

**Step 4: Verify compilation**

Run: `go build ./... && go test ./internal/bead/ -v`

**Step 5: Commit**

```bash
git add internal/runner/runner.go internal/bead/bead.go internal/bead/bead_test.go
git commit -m "feat: add epic completion trigger for thorough review"
```

---

### Task 14: Add `gromit review` CLI command

**Files:**
- Create: `cmd/gromit/review.go`

**Step 1: Write the command**

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/state"
	"github.com/spf13/cobra"
)

var (
	reviewNonInteractive bool
	reviewSince          string
	reviewEpic           string
	reviewDryRun         bool
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Run a thorough code review",
	Long: `Run a thorough review of recent changes.

Interactive mode (default): Launches a Claude Code session for collaborative review.
Non-interactive mode (--non-interactive): Runs autonomously, creates beads for issues found.

Scope options:
  --since <commit>   Review from a specific commit
  --epic <id>        Review changes from an epic's child beads
  (default)          Review since last thorough review`,
	RunE: runReview,
}

func init() {
	reviewCmd.Flags().BoolVar(&reviewNonInteractive, "non-interactive", false, "Run review autonomously without interactive session")
	reviewCmd.Flags().StringVar(&reviewSince, "since", "", "Review from this commit")
	reviewCmd.Flags().StringVar(&reviewEpic, "epic", "", "Review changes from this epic's beads")
	reviewCmd.Flags().BoolVar(&reviewDryRun, "dry-run", false, "Preview what would be reviewed")
	rootCmd.AddCommand(reviewCmd)
}
```

The `runReview` function:
1. Loads config and state
2. Determines scope (from commit)
3. Gets diff
4. If `--dry-run`, shows scope and exits
5. If interactive (default), writes prompt to temp file and launches `claude` interactively
6. If `--non-interactive`, runs the thorough review autonomously (same as automated trigger)

**Step 2: Interactive mode implementation**

```go
func runReviewInteractive(cfg *config.Config, fromCommit string) error {
	// Get diff
	diff, err := getGitDiffForReview(fromCommit)
	if err != nil {
		return fmt.Errorf("getting diff: %w", err)
	}

	// Build and render prompt
	// ... (use renderer to build ThoroughReviewContext and render)

	// Write prompt to temp file
	tmpFile, err := os.CreateTemp("", "gromit-review-*.md")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(renderedPrompt); err != nil {
		return fmt.Errorf("writing prompt: %w", err)
	}
	tmpFile.Close()

	// Launch interactive Claude session
	args := []string{"--prompt-file", tmpFile.Name()}
	args = append(args, cfg.Claude.Flags...)

	cmd := exec.Command(cfg.Claude.Binary, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
```

**Step 3: Verify compilation**

Run: `go build ./cmd/gromit/`

**Step 4: Commit**

```bash
git add cmd/gromit/review.go
git commit -m "feat: add gromit review command with interactive and non-interactive modes"
```

---

### Task 15: Initialize review state on first run

**Files:**
- Modify: `internal/runner/runner.go`

When starting the run loop, if `state.LastReviewCommit()` is empty, set it to the current HEAD. This establishes a baseline so the first thorough review has a starting point.

**Step 1: Add initialization**

In the `Run()` method, after loading state:

```go
// Initialize review baseline if not set
if sf.LastReviewCommit() == "" {
	currentCommit, err := getGitHead()
	if err == nil && currentCommit != "" {
		sf.RecordReview(currentCommit, 0)
		r.log("Initialized review baseline at commit %s", currentCommit[:8])
	}
}
```

**Step 2: Verify compilation and tests**

Run: `go build ./... && go test ./... -short`

**Step 3: Commit**

```bash
git add internal/runner/runner.go
git commit -m "feat: initialize review baseline commit on first run"
```

---

### Task 16: Update default gromit.yaml config with review section

**Files:**
- Modify: `cmd/gromit/init.go`

Add the review config section to `defaultConfig`:

```yaml
review:
  enabled: false               # set to true to enable post-iteration review
  model: sonnet
  match_build_model: true      # use opus if build used opus
  timeout: 120

  thorough:
    enabled: false             # set to true to enable periodic thorough reviews
    every_n_iterations: 5
    on_epic_complete: true
    model: opus
    timeout: 900
```

Default to disabled so existing users aren't surprised by new behavior.

**Step 1: Update the config template**

**Step 2: Verify compilation**

Run: `go build ./cmd/gromit/`

**Step 3: Commit**

```bash
git add cmd/gromit/init.go
git commit -m "feat: add review config section to default gromit.yaml"
```

---

### Task 17: Add import for review package in runner

**Files:**
- Modify: `internal/runner/runner.go`

Make sure the import for `"github.com/danabrams/gromit/internal/review"` is present. This may have been needed earlier but is called out explicitly in case it was missed.

Run: `go build ./...`

---

### Task 18: End-to-end manual test

Not a code task — manual verification steps:

1. Run `gromit init --force` in a test project to get new templates
2. Enable `review.enabled: true` in `gromit.yaml`
3. Run `gromit run -n 1` and verify the review step runs after validation
4. Run `gromit review --dry-run` to verify scope detection
5. Run `gromit review` to verify interactive mode launches
6. Run `gromit review --non-interactive` to verify autonomous mode

---

## Bead Dependency Graph

```
Task 1 (config) ─────────────────────────────┐
Task 2 (state) ──────────────────────────────┤
Task 3 (logger) ─────────────────────────────┤
Task 4 (review package) ────────────────────┤
Task 5 (prompt context) ───────────────────┤
Task 6 (review template) ──────────────────┤
Task 7 (thorough template) ────────────────┤
Task 8 (git diff helper) ─────────────────┼─── Task 9 (light review core)
                                            │       │
                                            │       ├── Task 10 (apply results)
                                            │       │       │
                                            │       │       └── Task 11 (integrate into processBead)
                                            │       │               │
                                            │       │               ├── Task 12 (N-iteration trigger)
                                            │       │               │
                                            │       │               └── Task 13 (epic trigger)
                                            │
                                            └────── Task 14 (gromit review CLI)
                                                        │
                                                        └── Task 15 (init baseline)

Task 16 (config template) — independent
Task 17 (imports) — dependent on Task 11
```

Tasks 1-8 can run in parallel. Tasks 9-13 are sequential. Task 14 can run in parallel with 9-13.
