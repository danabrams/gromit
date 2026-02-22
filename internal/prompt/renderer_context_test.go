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
