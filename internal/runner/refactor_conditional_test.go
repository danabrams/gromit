package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
)

// TestRefactorSkippedForFewFiles verifies that the refactor phase is skipped
// when a bead touches 2 or fewer files (below the default min_files_changed threshold).
func TestRefactorSkippedForFewFiles(t *testing.T) {
	// Expected failure: countChangedFiles function does not exist yet
	// Expected failure: shouldRunRefactor method does not exist on Runner
	// Expected failure: runRefactorPhase does not check file count before running

	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	writeTestFile(t, tmpDir, "file1.go", "initial content")
	commitGit(t, tmpDir, "initial commit")
	startCommit := getGitHeadCommit(t, tmpDir)

	// Make changes to exactly 2 files
	writeTestFile(t, tmpDir, "file1.go", "changed content")
	writeTestFile(t, tmpDir, "file2.go", "new file")
	commitGit(t, tmpDir, "changed 2 files")

	var buf strings.Builder
	refactorCalled := false

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			refactorCalled = true
			return "refactor prompt", nil
		},
	}

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled: false,
		},
	}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			// Return a diff showing 2 files changed
			return `diff --git a/file1.go b/file1.go
--- a/file1.go
+++ b/file1.go
@@ -1 +1 @@
-initial content
+changed content
diff --git a/file2.go b/file2.go
new file mode 100644
--- /dev/null
+++ b/file2.go
@@ -0,0 +1 @@
+new file`, nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1,
		},
		tier:        provider.TierMedium,
		model:       "sonnet",
		result:      &IterationResult{},
		startCommit: startCommit,
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify refactor was NOT called (2 files is below the threshold of 3)
	if refactorCalled {
		t.Error("runRefactorPhase() invoked refactor for 2-file change, but should have skipped it")
	}
}

// TestRefactorRunsForManyFiles verifies that the refactor phase runs
// when a bead touches 3 or more files (meets the default min_files_changed threshold).
func TestRefactorRunsForManyFiles(t *testing.T) {
	// Expected failure: countChangedFiles function does not exist yet
	// Expected failure: shouldRunRefactor method does not exist on Runner
	// Expected failure: runRefactorPhase does not check file count before running

	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	writeTestFile(t, tmpDir, "file1.go", "initial content")
	commitGit(t, tmpDir, "initial commit")
	startCommit := getGitHeadCommit(t, tmpDir)

	// Make changes to exactly 3 files (meets threshold)
	writeTestFile(t, tmpDir, "file1.go", "changed content")
	writeTestFile(t, tmpDir, "file2.go", "new file")
	writeTestFile(t, tmpDir, "file3.go", "another file")
	commitGit(t, tmpDir, "changed 3 files")

	var buf strings.Builder
	refactorCalled := false

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			refactorCalled = true
			return "refactor prompt", nil
		},
	}

	mockProvider := &mockProviderForRefactor{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  "refactor complete",
			}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "validation-model",
				Output:  "VALIDATION_PASSED",
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
		},
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			// Return a diff showing 3 files changed
			return `diff --git a/file1.go b/file1.go
--- a/file1.go
+++ b/file1.go
@@ -1 +1 @@
-initial content
+changed content
diff --git a/file2.go b/file2.go
new file mode 100644
--- /dev/null
+++ b/file2.go
@@ -0,0 +1 @@
+new file
diff --git a/file3.go b/file3.go
new file mode 100644
--- /dev/null
+++ b/file3.go
@@ -0,0 +1 @@
+another file`, nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1,
		},
		tier:        provider.TierMedium,
		model:       "sonnet",
		result:      &IterationResult{},
		startCommit: startCommit,
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify refactor WAS called (3 files meets the threshold)
	if !refactorCalled {
		t.Error("runRefactorPhase() skipped refactor for 3-file change, but should have run it")
	}
}

// TestRefactorSkippedForHaikuTier verifies that the refactor phase is skipped
// when the bead tier is "low" (haiku), regardless of file count.
func TestRefactorSkippedForHaikuTier(t *testing.T) {
	// Expected failure: shouldRunRefactor method does not exist on Runner
	// Expected failure: runRefactorPhase does not check tier before running
	// Expected failure: tier-based refactor gating logic not implemented

	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	writeTestFile(t, tmpDir, "file1.go", "initial content")
	commitGit(t, tmpDir, "initial commit")
	startCommit := getGitHeadCommit(t, tmpDir)

	// Make changes to many files (well above threshold)
	writeTestFile(t, tmpDir, "file1.go", "changed")
	writeTestFile(t, tmpDir, "file2.go", "new")
	writeTestFile(t, tmpDir, "file3.go", "new")
	writeTestFile(t, tmpDir, "file4.go", "new")
	writeTestFile(t, tmpDir, "file5.go", "new")
	commitGit(t, tmpDir, "changed 5 files")

	var buf strings.Builder
	refactorCalled := false

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			refactorCalled = true
			return "refactor prompt", nil
		},
	}

	cfg := &config.Config{
		Validation: config.ValidationConfig{
			Enabled: false,
		},
	}
	cfg.SetDefaults()

	r := &Runner{
		cfg:      cfg,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			return `diff --git a/file1.go b/file1.go
diff --git a/file2.go b/file2.go
diff --git a/file3.go b/file3.go
diff --git a/file4.go b/file4.go
diff --git a/file5.go b/file5.go`, nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 2, // P2 = haiku
		},
		tier:        provider.TierLow, // haiku tier
		model:       "haiku",
		result:      &IterationResult{},
		startCommit: startCommit,
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify refactor was NOT called (haiku tier should skip refactoring)
	if refactorCalled {
		t.Error("runRefactorPhase() invoked refactor for haiku-tier bead, but should have skipped it")
	}
}

// TestRefactorRunsForSonnetTier verifies that the refactor phase runs
// when the bead tier is "medium" (sonnet) and file count meets threshold.
func TestRefactorRunsForSonnetTier(t *testing.T) {
	// Expected failure: shouldRunRefactor method does not exist on Runner
	// Expected failure: tier-based refactor gating logic not implemented

	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	writeTestFile(t, tmpDir, "file1.go", "initial content")
	commitGit(t, tmpDir, "initial commit")
	startCommit := getGitHeadCommit(t, tmpDir)

	// Make changes to 3 files
	writeTestFile(t, tmpDir, "file1.go", "changed")
	writeTestFile(t, tmpDir, "file2.go", "new")
	writeTestFile(t, tmpDir, "file3.go", "new")
	commitGit(t, tmpDir, "changed 3 files")

	var buf strings.Builder
	refactorCalled := false

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			refactorCalled = true
			return "refactor prompt", nil
		},
	}

	mockProvider := &mockProviderForRefactor{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  "refactor complete",
			}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "validation-model",
				Output:  "VALIDATION_PASSED",
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
		},
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			return `diff --git a/file1.go b/file1.go
diff --git a/file2.go b/file2.go
diff --git a/file3.go b/file3.go`, nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1, // P1 = sonnet
		},
		tier:        provider.TierMedium, // sonnet tier
		model:       "sonnet",
		result:      &IterationResult{},
		startCommit: startCommit,
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify refactor WAS called (sonnet tier with 3+ files should run)
	if !refactorCalled {
		t.Error("runRefactorPhase() skipped refactor for sonnet-tier bead with 3 files, but should have run it")
	}
}

// TestRefactorConfigMinFilesChanged verifies that the refactor.min_files_changed
// config option controls the file count threshold for running refactor.
func TestRefactorConfigMinFilesChanged(t *testing.T) {
	// Expected failure: config.RefactorConfig type does not exist yet
	// Expected failure: config.RefactorConfig.MinFilesChanged field does not exist
	// Expected failure: Config.Refactor field does not exist
	// Expected failure: shouldRunRefactor does not check cfg.Refactor.MinFilesChanged

	tests := []struct {
		name              string
		minFilesChanged   int
		filesChanged      int
		expectRefactorRun bool
	}{
		{
			name:              "below custom threshold",
			minFilesChanged:   5,
			filesChanged:      4,
			expectRefactorRun: false,
		},
		{
			name:              "meets custom threshold",
			minFilesChanged:   5,
			filesChanged:      5,
			expectRefactorRun: true,
		},
		{
			name:              "above custom threshold",
			minFilesChanged:   5,
			filesChanged:      6,
			expectRefactorRun: true,
		},
		{
			name:              "threshold of 1",
			minFilesChanged:   1,
			filesChanged:      1,
			expectRefactorRun: true,
		},
		{
			name:              "threshold of 1 with 0 files",
			minFilesChanged:   1,
			filesChanged:      0,
			expectRefactorRun: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			setupGitRepo(t, tmpDir)
			writeTestFile(t, tmpDir, "file1.go", "initial")
			commitGit(t, tmpDir, "initial")
			startCommit := getGitHeadCommit(t, tmpDir)

			// Build a diff string with the specified number of files
			var diffParts []string
			for i := 1; i <= tt.filesChanged; i++ {
				diffParts = append(diffParts, "diff --git a/file"+string(rune('0'+i))+".go b/file"+string(rune('0'+i))+".go")
			}
			mockDiff := strings.Join(diffParts, "\n")

			var buf strings.Builder
			refactorCalled := false

			mockRend := &mockPromptRenderer{
				RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
					refactorCalled = true
					return "refactor prompt", nil
				},
			}

			mockProvider := &mockProviderForRefactor{
				name: "test-provider",
				runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
					return &provider.Result{
						Success: true,
						Model:   "sonnet",
						Output:  "refactor complete",
					}, nil
				},
				runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
					return &provider.Result{
						Success: true,
						Model:   "validation-model",
						Output:  "VALIDATION_PASSED",
					}, nil
				},
			}

			mockRouter := provider.NewSingleProviderRouter(mockProvider)

			r := &Runner{
				cfg: &config.Config{
					Refactor: config.RefactorConfig{
						MinFilesChanged: tt.minFilesChanged,
					},
					Validation: config.ValidationConfig{
						Enabled:  true,
						Commands: []string{"go test ./..."},
					},
				},
				router:   mockRouter,
				renderer: mockRend,
				output:   &buf,
				gitDiffFn: func(commit string) (string, error) {
					return mockDiff, nil
				},
			}

			bc := &beadContext{
				bead: &bead.Bead{
					ID:       "test-1",
					Title:    "Test",
					Priority: 1, // sonnet tier
				},
				tier:        provider.TierMedium,
				model:       "sonnet",
				result:      &IterationResult{},
				startCommit: startCommit,
				promptCtx: &prompt.Context{
					WorkDir: tmpDir,
				},
			}

			oldDir := changeDir(t, tmpDir)
			defer changeDir(t, oldDir)

			err := r.runRefactorPhase(context.Background(), bc)
			if err != nil {
				t.Fatalf("runRefactorPhase() error = %v", err)
			}

			if refactorCalled != tt.expectRefactorRun {
				t.Errorf("runRefactorPhase() refactor called = %v, want %v (min=%d, files=%d)",
					refactorCalled, tt.expectRefactorRun, tt.minFilesChanged, tt.filesChanged)
			}
		})
	}
}

// TestRefactorConfigDefaultMinFilesChanged verifies that when refactor.min_files_changed
// is not explicitly set in the config, it defaults to 3.
func TestRefactorConfigDefaultMinFilesChanged(t *testing.T) {
	// Expected failure: config.RefactorConfig type does not exist yet
	// Expected failure: config.RefactorConfig.MinFilesChanged field does not exist
	// Expected failure: SetDefaults does not set default for MinFilesChanged

	cfg := &config.Config{}
	cfg.SetDefaults()

	// Verify default is 3
	if cfg.Refactor.MinFilesChanged != 3 {
		t.Errorf("Default MinFilesChanged = %d, want 3", cfg.Refactor.MinFilesChanged)
	}
}

// TestRefactorConfigMinFilesChangedZeroMeansDisabled verifies that setting
// min_files_changed to 0 effectively disables the file count check (always runs
// refactor for non-haiku tiers).
func TestRefactorConfigMinFilesChangedZeroMeansDisabled(t *testing.T) {
	// Expected failure: shouldRunRefactor does not treat 0 as special case
	// Expected failure: min_files_changed=0 handling not implemented

	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)
	writeTestFile(t, tmpDir, "file1.go", "initial")
	commitGit(t, tmpDir, "initial")
	startCommit := getGitHeadCommit(t, tmpDir)

	var buf strings.Builder
	refactorCalled := false

	mockRend := &mockPromptRenderer{
		RenderRefactorFn: func(ctx *prompt.Context) (string, error) {
			refactorCalled = true
			return "refactor prompt", nil
		},
	}

	mockProvider := &mockProviderForRefactor{
		name: "test-provider",
		runFn: func(ctx context.Context, prompt, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "sonnet",
				Output:  "refactor complete",
			}, nil
		},
		runValidationFn: func(ctx context.Context, commands []string, tier string, workDir string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Model:   "validation-model",
				Output:  "VALIDATION_PASSED",
			}, nil
		},
	}

	mockRouter := provider.NewSingleProviderRouter(mockProvider)

	r := &Runner{
		cfg: &config.Config{
			Refactor: config.RefactorConfig{
				MinFilesChanged: 0, // 0 means always run (no file count check)
			},
			Validation: config.ValidationConfig{
				Enabled:  true,
				Commands: []string{"go test ./..."},
			},
		},
		router:   mockRouter,
		renderer: mockRend,
		output:   &buf,
		gitDiffFn: func(commit string) (string, error) {
			// Return diff with only 1 file (below normal threshold)
			return "diff --git a/file1.go b/file1.go", nil
		},
	}

	bc := &beadContext{
		bead: &bead.Bead{
			ID:       "test-1",
			Title:    "Test",
			Priority: 1, // sonnet tier
		},
		tier:        provider.TierMedium,
		model:       "sonnet",
		result:      &IterationResult{},
		startCommit: startCommit,
		promptCtx: &prompt.Context{
			WorkDir: tmpDir,
		},
	}

	oldDir := changeDir(t, tmpDir)
	defer changeDir(t, oldDir)

	err := r.runRefactorPhase(context.Background(), bc)
	if err != nil {
		t.Fatalf("runRefactorPhase() error = %v", err)
	}

	// Verify refactor WAS called (min_files_changed=0 means always run)
	if !refactorCalled {
		t.Error("runRefactorPhase() skipped refactor when min_files_changed=0, but should have run it")
	}
}

// TestCountChangedFilesFromDiff verifies that countChangedFiles correctly
// counts the number of files in a git diff output.
func TestCountChangedFilesFromDiff(t *testing.T) {
	// Expected failure: countChangedFiles function does not exist yet

	tests := []struct {
		name      string
		diff      string
		wantCount int
	}{
		{
			name:      "empty diff",
			diff:      "",
			wantCount: 0,
		},
		{
			name: "single file",
			diff: `diff --git a/file1.go b/file1.go
index 1234567..abcdefg 100644
--- a/file1.go
+++ b/file1.go
@@ -1 +1 @@
-old content
+new content`,
			wantCount: 1,
		},
		{
			name: "multiple files",
			diff: `diff --git a/file1.go b/file1.go
--- a/file1.go
+++ b/file1.go
@@ -1 +1 @@
diff --git a/file2.go b/file2.go
--- a/file2.go
+++ b/file2.go
diff --git a/file3.go b/file3.go
--- a/file3.go
+++ b/file3.go`,
			wantCount: 3,
		},
		{
			name: "new files",
			diff: `diff --git a/new1.go b/new1.go
new file mode 100644
--- /dev/null
+++ b/new1.go
diff --git a/new2.go b/new2.go
new file mode 100644
--- /dev/null
+++ b/new2.go`,
			wantCount: 2,
		},
		{
			name: "deleted files",
			diff: `diff --git a/deleted.go b/deleted.go
deleted file mode 100644
--- a/deleted.go
+++ /dev/null`,
			wantCount: 1,
		},
		{
			name: "mixed operations",
			diff: `diff --git a/modified.go b/modified.go
--- a/modified.go
+++ b/modified.go
diff --git a/new.go b/new.go
new file mode 100644
--- /dev/null
+++ b/new.go
diff --git a/deleted.go b/deleted.go
deleted file mode 100644
--- a/deleted.go
+++ /dev/null`,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countChangedFiles(tt.diff)
			if got != tt.wantCount {
				t.Errorf("countChangedFiles() = %d, want %d", got, tt.wantCount)
			}
		})
	}
}
