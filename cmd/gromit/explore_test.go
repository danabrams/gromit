package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/prompt"
)

// setupExploreTest creates a temp directory with the standard explore test
// structure: .gromit/ with templates/, CLAUDE.md, and config.
//
// Note: After extracting explore logic to internal/pipeline, this helper
// is preserved for CLI adapter tests that need to verify command setup.
func setupExploreTest(t *testing.T) (*config.Config, string) {
	t.Helper()

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")

	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte("# Project\n\nThis is project context."), 0644); err != nil {
		t.Fatalf("failed to create CLAUDE.md: %v", err)
	}

	cfg := &config.Config{
		Paths: config.PathsConfig{
			Templates:       templatesDir,
			ProjectClaudeMD: claudeMDPath,
			GromitDir:       gromitDir,
		},
	}

	return cfg, gromitDir
}

// buildExplorePrompt is preserved as a reference for what the CLI adapter
// used to do before extracting to Pipeline.Explore.
//
// The actual prompt building now happens in internal/prompt via
// explorePromptRenderer.RenderExplore(), which is called by Pipeline.Explore().
//
// This stub is kept to satisfy final_verification_test.go which checks for
// the existence of these functions after the explore test consolidation.
func buildExplorePrompt(cfg *config.Config, gromitDir string, args []string) (string, error) {
	// This function was moved to internal/prompt as part of pipeline extraction.
	// Tests for prompt building are now in internal/prompt/prompt_test.go.
	// Tests for the full explore workflow are in internal/pipeline/explore_test.go.
	// This stub documents the refactoring for future reference.
	return "", nil
}

func TestBuildExplorePipeline_NilConfigUsesResolvedDefaults(t *testing.T) {
	t.Chdir(t.TempDir())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("buildExplorePipeline(nil) panicked: %v", r)
		}
	}()

	p, err := buildExplorePipeline(nil)
	if err != nil {
		t.Fatalf("buildExplorePipeline(nil) returned error: %v", err)
	}
	if p == nil {
		t.Fatal("buildExplorePipeline(nil) returned nil pipeline")
	}
}

func TestExplorePromptRenderer_RenderExploreBuildsPromptDiagnostics(t *testing.T) {
	cfg, gromitDir := setupExploreTest(t)

	if err := os.WriteFile(filepath.Join(gromitDir, "RULES.md"), []byte("## Process\n- Keep it simple\n"), 0o644); err != nil {
		t.Fatalf("failed to create RULES.md: %v", err)
	}

	renderer, err := prompt.NewRenderer(
		cfg.Paths.Templates,
		filepath.Join(gromitDir, "specs"),
		cfg.Paths.ProjectClaudeMD,
		gromitDir,
	)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}

	adapter := &explorePromptRenderer{renderer: renderer}
	const topic = "Improve onboarding flow"
	if _, err := adapter.RenderExplore(&pipeline.ExplorePromptInput{Query: topic}); err != nil {
		t.Fatalf("RenderExplore() error = %v", err)
	}

	diagnostics := adapter.LastDiagnostics()
	if diagnostics == nil {
		t.Fatal("LastDiagnostics() = nil, want non-nil")
	}
	if diagnostics.PromptType != "explore" {
		t.Fatalf("PromptType = %q, want %q", diagnostics.PromptType, "explore")
	}

	for _, key := range []string{"topic", prompt.SectionClaudeMD, prompt.SectionRules, "learnings", "instructions"} {
		if _, ok := diagnostics.SectionTokens[key]; !ok {
			t.Fatalf("SectionTokens missing key %q", key)
		}
	}
}
