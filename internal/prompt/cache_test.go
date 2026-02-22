package prompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
)

// TestLoadClaudeMD_CachesOnFirstCall verifies that LoadClaudeMD reads from disk once
// and returns cached content on subsequent calls.
func TestLoadClaudeMD_CachesOnFirstCall(t *testing.T) {
	// Expected failure: Renderer does not have a cache field or caching logic for CLAUDE.md yet
	tmpDir := t.TempDir()
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")

	// Write initial content
	initialContent := "# Initial CLAUDE.md content"
	if err := os.WriteFile(claudeMDPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	r := &Renderer{claudeMDPath: claudeMDPath}

	// First call should read from disk
	content1, err := r.LoadClaudeMD()
	if err != nil {
		t.Fatalf("LoadClaudeMD() first call error = %v", err)
	}
	if content1 != initialContent {
		t.Errorf("LoadClaudeMD() first call = %q, want %q", content1, initialContent)
	}

	// Modify file on disk AFTER first load
	modifiedContent := "# Modified CLAUDE.md content"
	if err := os.WriteFile(claudeMDPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Second call should return cached content (original content, not modified)
	content2, err := r.LoadClaudeMD()
	if err != nil {
		t.Fatalf("LoadClaudeMD() second call error = %v", err)
	}
	if content2 != initialContent {
		t.Errorf("LoadClaudeMD() second call = %q, want cached %q (got fresh read instead)", content2, initialContent)
	}
}

// TestLoadRules_CachesOnFirstCall verifies that LoadRules reads from disk once
// and returns cached content on subsequent calls.
func TestLoadRules_CachesOnFirstCall(t *testing.T) {
	// Expected failure: Renderer does not have a cache field or caching logic for RULES.md yet
	tmpDir := t.TempDir()
	rulesPath := filepath.Join(tmpDir, "RULES.md")

	// Write initial content
	initialContent := "# Initial Rules\n\n- Rule 1\n- Rule 2"
	if err := os.WriteFile(rulesPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	r := &Renderer{rulesPath: rulesPath}

	// First call should read from disk
	content1, err := r.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() first call error = %v", err)
	}
	if content1 != initialContent {
		t.Errorf("LoadRules() first call = %q, want %q", content1, initialContent)
	}

	// Modify file on disk AFTER first load
	modifiedContent := "# Modified Rules\n\n- Rule 3"
	if err := os.WriteFile(rulesPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}

	// Second call should return cached content (original content, not modified)
	content2, err := r.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() second call error = %v", err)
	}
	if content2 != initialContent {
		t.Errorf("LoadRules() second call = %q, want cached %q (got fresh read instead)", content2, initialContent)
	}
}

// TestLoadSpec_CachesPerSpecName verifies that LoadSpec caches each spec file
// independently by spec name and returns cached content on subsequent calls.
func TestLoadSpec_CachesPerSpecName(t *testing.T) {
	// Expected failure: Renderer does not have a cache field or caching logic for spec files yet
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	r := &Renderer{specsDir: specsDir}

	// Create two spec files
	spec1Content := "# Auth Spec\n\nAuthentication requirements"
	spec2Content := "# Payment Spec\n\nPayment processing requirements"

	spec1Path := filepath.Join(specsDir, "auth.md")
	spec2Path := filepath.Join(specsDir, "payment.md")

	if err := os.WriteFile(spec1Path, []byte(spec1Content), 0644); err != nil {
		t.Fatalf("failed to write auth spec: %v", err)
	}
	if err := os.WriteFile(spec2Path, []byte(spec2Content), 0644); err != nil {
		t.Fatalf("failed to write payment spec: %v", err)
	}

	// Load first spec
	content1, err := r.LoadSpec("auth")
	if err != nil {
		t.Fatalf("LoadSpec(auth) first call error = %v", err)
	}
	if content1 != spec1Content {
		t.Errorf("LoadSpec(auth) first call = %q, want %q", content1, spec1Content)
	}

	// Load second spec
	content2, err := r.LoadSpec("payment")
	if err != nil {
		t.Fatalf("LoadSpec(payment) first call error = %v", err)
	}
	if content2 != spec2Content {
		t.Errorf("LoadSpec(payment) first call = %q, want %q", content2, spec2Content)
	}

	// Modify both specs on disk
	modifiedSpec1 := "# Auth Spec Modified"
	modifiedSpec2 := "# Payment Spec Modified"

	if err := os.WriteFile(spec1Path, []byte(modifiedSpec1), 0644); err != nil {
		t.Fatalf("failed to modify auth spec: %v", err)
	}
	if err := os.WriteFile(spec2Path, []byte(modifiedSpec2), 0644); err != nil {
		t.Fatalf("failed to modify payment spec: %v", err)
	}

	// Load both specs again - should return cached content
	cachedContent1, err := r.LoadSpec("auth")
	if err != nil {
		t.Fatalf("LoadSpec(auth) second call error = %v", err)
	}
	if cachedContent1 != spec1Content {
		t.Errorf("LoadSpec(auth) second call = %q, want cached %q (got fresh read instead)", cachedContent1, spec1Content)
	}

	cachedContent2, err := r.LoadSpec("payment")
	if err != nil {
		t.Fatalf("LoadSpec(payment) second call error = %v", err)
	}
	if cachedContent2 != spec2Content {
		t.Errorf("LoadSpec(payment) second call = %q, want cached %q (got fresh read instead)", cachedContent2, spec2Content)
	}
}

// TestBuildContext_UsesCachedFiles verifies that BuildContext uses cached
// CLAUDE.md and RULES.md content rather than reading from disk multiple times.
func TestBuildContext_UsesCachedFiles(t *testing.T) {
	// Expected failure: BuildContext calls LoadClaudeMD/LoadRules which don't cache yet
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	templatesDir := filepath.Join(gromitDir, "templates")
	specsDir := filepath.Join(gromitDir, "specs")

	if err := os.MkdirAll(gromitDir, 0755); err != nil {
		t.Fatalf("failed to create gromit dir: %v", err)
	}
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.MkdirAll(specsDir, 0755); err != nil {
		t.Fatalf("failed to create specs dir: %v", err)
	}

	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	rulesPath := filepath.Join(gromitDir, "RULES.md")

	// Write initial content
	initialClaudeMD := "# Project Context"
	initialRules := "# Project Rules"

	if err := os.WriteFile(claudeMDPath, []byte(initialClaudeMD), 0644); err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}
	if err := os.WriteFile(rulesPath, []byte(initialRules), 0644); err != nil {
		t.Fatalf("failed to write RULES.md: %v", err)
	}

	// Write empty LEARNINGS.md to avoid errors
	learningsPath := filepath.Join(gromitDir, "LEARNINGS.md")
	if err := os.WriteFile(learningsPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write LEARNINGS.md: %v", err)
	}

	r, err := NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}

	testBead := &bead.Bead{
		ID:          "test-1",
		Title:       "Test task",
		Description: "Test description",
		Priority:    1,
	}

	// Build context first time
	ctx1, err := r.BuildContext(testBead, nil, 1, "sonnet", promptPhaseBuild)
	if err != nil {
		t.Fatalf("BuildContext() first call error = %v", err)
	}
	if ctx1.ClaudeMD != initialClaudeMD {
		t.Errorf("BuildContext() first call ClaudeMD = %q, want %q", ctx1.ClaudeMD, initialClaudeMD)
	}
	if ctx1.Rules != initialRules {
		t.Errorf("BuildContext() first call Rules = %q, want %q", ctx1.Rules, initialRules)
	}

	// Modify files on disk
	modifiedClaudeMD := "# Modified Project Context"
	modifiedRules := "# Modified Project Rules"

	if err := os.WriteFile(claudeMDPath, []byte(modifiedClaudeMD), 0644); err != nil {
		t.Fatalf("failed to modify CLAUDE.md: %v", err)
	}
	if err := os.WriteFile(rulesPath, []byte(modifiedRules), 0644); err != nil {
		t.Fatalf("failed to modify RULES.md: %v", err)
	}

	// Build context second time - should use cached content
	ctx2, err := r.BuildContext(testBead, nil, 2, "sonnet", promptPhaseBuild)
	if err != nil {
		t.Fatalf("BuildContext() second call error = %v", err)
	}
	if ctx2.ClaudeMD != initialClaudeMD {
		t.Errorf("BuildContext() second call ClaudeMD = %q, want cached %q (got fresh read)", ctx2.ClaudeMD, initialClaudeMD)
	}
	if ctx2.Rules != initialRules {
		t.Errorf("BuildContext() second call Rules = %q, want cached %q (got fresh read)", ctx2.Rules, initialRules)
	}
}

// TestMultipleRenderers_IndependentCaches verifies that different Renderer
// instances maintain independent caches.
func TestMultipleRenderers_IndependentCaches(t *testing.T) {
	// Expected failure: Renderer does not have per-instance cache fields yet
	tmpDir := t.TempDir()
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")

	// Write initial content
	initialContent := "# Initial content"
	if err := os.WriteFile(claudeMDPath, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create first renderer and load
	r1 := &Renderer{claudeMDPath: claudeMDPath}
	content1, err := r1.LoadClaudeMD()
	if err != nil {
		t.Fatalf("r1.LoadClaudeMD() error = %v", err)
	}
	if content1 != initialContent {
		t.Errorf("r1.LoadClaudeMD() = %q, want %q", content1, initialContent)
	}

	// Modify file
	modifiedContent := "# Modified content"
	if err := os.WriteFile(claudeMDPath, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// Create second renderer - should get modified content
	r2 := &Renderer{claudeMDPath: claudeMDPath}
	content2, err := r2.LoadClaudeMD()
	if err != nil {
		t.Fatalf("r2.LoadClaudeMD() error = %v", err)
	}
	if content2 != modifiedContent {
		t.Errorf("r2.LoadClaudeMD() = %q, want %q (should read fresh, not use r1's cache)", content2, modifiedContent)
	}

	// First renderer should still return cached original content
	cachedContent1, err := r1.LoadClaudeMD()
	if err != nil {
		t.Fatalf("r1.LoadClaudeMD() second call error = %v", err)
	}
	if cachedContent1 != initialContent {
		t.Errorf("r1.LoadClaudeMD() second call = %q, want cached %q", cachedContent1, initialContent)
	}
}
