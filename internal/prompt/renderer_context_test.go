package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

func newRendererForContextTests(t *testing.T, rules string, claude string, specName string, specBody string) *Renderer {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("MkdirAll(specs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte(rules), 0644); err != nil {
		t.Fatalf("WriteFile(RULES.md) error = %v", err)
	}
	claudePath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte(claude), 0644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md) error = %v", err)
	}
	if strings.TrimSpace(specName) != "" {
		if err := os.WriteFile(filepath.Join(specsDir, specName+".md"), []byte(specBody), 0644); err != nil {
			t.Fatalf("WriteFile(spec) error = %v", err)
		}
	}

	r, err := NewRenderer("", specsDir, claudePath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
	return r
}

func TestBuildContext_RedPhasePrunesProjectContext(t *testing.T) {
	r := newRendererForContextTests(t,
		"## Red <!-- phases: red -->\nred-rules\n\n## Build <!-- phases: build -->\nbuild-rules\n",
		"# Claude\nfull project context",
		"phase-cost-optimization",
		"# Spec\ncriterion",
	)

	ctx, err := r.BuildContext(&bead.Bead{
		ID:     "b1",
		Title:  "t",
		Labels: []string{"spec:phase-cost-optimization"},
	}, nil, 1, "sonnet", "red")
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}

	if ctx.ClaudeMD != "" {
		t.Fatalf("BuildContext(red) should omit ClaudeMD, got %q", ctx.ClaudeMD)
	}
	if strings.Contains(ctx.Rules, "build-rules") {
		t.Fatalf("BuildContext(red) should omit build-only rules, got %q", ctx.Rules)
	}
	if !strings.Contains(ctx.Rules, "red-rules") {
		t.Fatalf("BuildContext(red) should keep red rules, got %q", ctx.Rules)
	}
	if ctx.Spec == "" {
		t.Fatal("BuildContext(red) should retain spec context")
	}
}

func TestBuildContext_EmptyPhasePreservesFullContext(t *testing.T) {
	r := newRendererForContextTests(t,
		"## Build <!-- phases: build -->\nbuild-rules\n",
		"# Claude\nfull project context",
		"phase-cost-optimization",
		"# Spec\ncriterion",
	)

	ctx, err := r.BuildContext(&bead.Bead{
		ID:     "b1",
		Title:  "t",
		Labels: []string{"spec:phase-cost-optimization"},
	}, nil, 1, "sonnet", "")
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}

	if ctx.ClaudeMD == "" {
		t.Fatal("BuildContext(empty phase) should preserve ClaudeMD")
	}
	if !strings.Contains(ctx.Rules, "build-rules") {
		t.Fatalf("BuildContext(empty phase) should preserve rules, got %q", ctx.Rules)
	}
	if ctx.Spec == "" {
		t.Fatal("BuildContext(empty phase) should preserve spec context")
	}
}

func TestBuildContext_RedPhaseScopesBeforeBudgetShaping(t *testing.T) {
	r := setupRendererWithLearnings(t, 3, 120)
	r.SetBudgetConfig(40, 40)

	ctx, err := r.BuildContext(newBuildContextTestBead(), nil, 1, "sonnet", "red")
	if err != nil {
		t.Fatalf("BuildContext() error = %v", err)
	}
	if len(ctx.ConfirmedLearnings) != 0 {
		t.Fatalf("BuildContext(red) should remove ConfirmedLearnings before budget shaping, got %d", len(ctx.ConfirmedLearnings))
	}

	_, report := r.shapeBuildContext(ctx, "red")
	if report == nil {
		t.Fatal("expected shape report")
	}
	if hasTrimAction(report.TrimActions, trimCapConfirmedLearnings) {
		t.Fatalf("budget shaping should not cap red-phase ConfirmedLearnings once phase scoping is applied, got %v", report.TrimActions)
	}
	if hasTrimAction(report.TrimActions, trimDropConfirmedLearnings) {
		t.Fatalf("budget shaping should not drop red-phase ConfirmedLearnings once phase scoping is applied, got %v", report.TrimActions)
	}
}

func TestBuildContext_PopulatesStaticPreambleCacheMetadata(t *testing.T) {
	r := newRendererForContextTests(t,
		"## Build <!-- phases: build -->\nbuild-rules\n",
		"# Claude\nproject context",
		"phase-cost-optimization",
		"# Spec\ncriterion",
	)
	b := &bead.Bead{
		ID:     "b1",
		Title:  "task",
		Labels: []string{"spec:phase-cost-optimization"},
	}

	ctx1, err := r.BuildContext(b, nil, 1, "sonnet", "build")
	if err != nil {
		t.Fatalf("BuildContext() first call error = %v", err)
	}
	ctx2, err := r.BuildContext(b, nil, 2, "haiku", "build")
	if err != nil {
		t.Fatalf("BuildContext() second call error = %v", err)
	}

	if ctx1.StaticPreambleCacheClass == "" || ctx1.StaticPreambleCacheKey == "" {
		t.Fatalf("expected cache metadata to be populated, got class=%q key=%q", ctx1.StaticPreambleCacheClass, ctx1.StaticPreambleCacheKey)
	}
	if ctx1.StaticPreambleCacheClass != ctx2.StaticPreambleCacheClass {
		t.Fatalf("expected cache class to remain stable, got %q vs %q", ctx1.StaticPreambleCacheClass, ctx2.StaticPreambleCacheClass)
	}
	if ctx1.StaticPreambleCacheKey != ctx2.StaticPreambleCacheKey {
		t.Fatalf("expected cache key to remain stable across dynamic BuildContext fields, got %q vs %q", ctx1.StaticPreambleCacheKey, ctx2.StaticPreambleCacheKey)
	}
}

func TestBuildContext_SkipBuildLearningsOnlyForBuildPhase(t *testing.T) {
	r := setupRendererWithLearnings(t, 2, 120)
	r.SetSkipBuildLearnings(true)
	testBead := newBuildContextTestBead()

	buildCtx, err := r.BuildContext(testBead, nil, 1, "sonnet", promptPhaseBuild)
	if err != nil {
		t.Fatalf("BuildContext(build) error = %v", err)
	}
	if len(buildCtx.ConfirmedLearnings) != 0 {
		t.Fatalf("expected build-phase context to exclude confirmed learnings, got %d", len(buildCtx.ConfirmedLearnings))
	}

	reviewCtx, err := r.BuildContext(testBead, nil, 1, "sonnet", "review")
	if err != nil {
		t.Fatalf("BuildContext(review) error = %v", err)
	}
	if len(reviewCtx.ConfirmedLearnings) == 0 {
		t.Fatal("expected review-phase context to include confirmed learnings")
	}
}
