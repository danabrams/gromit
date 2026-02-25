package pipeline

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
)

// testBeadQueryClientStatus is a mock BeadQueryClient for status tests
type testBeadQueryClientStatus struct {
	readyBeads   []string
	inProgress   int
	deferred     int
	closed       int
	open         int
	closedAfter  int
	countByError map[string]error
}

func (m *testBeadQueryClientStatus) CountByStatus(status string) (int, error) {
	if m.countByError != nil {
		if err, ok := m.countByError[status]; ok {
			return 0, err
		}
	}
	switch status {
	case "in_progress":
		return m.inProgress, nil
	case "deferred":
		return m.deferred, nil
	case "closed":
		return m.closed, nil
	case "open":
		return m.open, nil
	default:
		return 0, nil
	}
}

func (m *testBeadQueryClientStatus) ListReadyIDs() ([]string, error) {
	return m.readyBeads, nil
}

func (m *testBeadQueryClientStatus) CountClosedAfter(since time.Time) (int, error) {
	return m.closedAfter, nil
}

func disableLiveBDForStatusTests(t *testing.T) {
	t.Helper()

	// Keep unit tests deterministic by ensuring ReadStatus cannot invoke a live bd CLI.
	t.Setenv("PATH", t.TempDir())
}

func TestReadStatus(t *testing.T) {
	disableLiveBDForStatusTests(t)

	tests := []struct {
		name               string
		setupFunc          func(t *testing.T, tmpDir string)
		wantUnrefinedCount int
		wantUnplannedCount int
		wantUndecomposed   int
		wantRecommendation string
	}{
		{
			name: "empty directories",
			setupFunc: func(t *testing.T, tmpDir string) {
				// Create empty directories
				os.MkdirAll(filepath.Join(tmpDir, ".gromit"), 0755)
				os.MkdirAll(filepath.Join(tmpDir, ".gromit", "specs"), 0755)
				os.MkdirAll(filepath.Join(tmpDir, ".gromit", "plans"), 0755)
			},
			wantUnrefinedCount: 0,
			wantUnplannedCount: 0,
			wantUndecomposed:   0,
			wantRecommendation: "No work in pipeline",
		},
		{
			name: "unrefined ideas only",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(filepath.Join(gromitDir, "specs"), 0755)
				os.MkdirAll(filepath.Join(gromitDir, "plans"), 0755)

				// Add unrefined ideas
				bf, _ := backlog.NewFile(gromitDir)
				bf.Add(&backlog.Idea{
					ID:     "idea-1",
					Text:   "Add user authentication",
					Status: "",
				})
				bf.Add(&backlog.Idea{
					ID:     "idea-2",
					Text:   "Implement dark mode",
					Status: "",
				})
			},
			wantUnrefinedCount: 2,
			wantUnplannedCount: 0,
			wantUndecomposed:   0,
			wantRecommendation: "Refine idea: Add user authentication",
		},
		{
			name: "refined ideas not counted as unrefined",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(filepath.Join(gromitDir, "specs"), 0755)
				os.MkdirAll(filepath.Join(gromitDir, "plans"), 0755)

				bf, _ := backlog.NewFile(gromitDir)
				bf.Add(&backlog.Idea{
					ID:       "idea-1",
					Text:     "Add user authentication",
					Status:   "refined",
					SpecName: "user-auth",
				})
				bf.Add(&backlog.Idea{
					ID:     "idea-2",
					Text:   "Implement dark mode",
					Status: "",
				})
			},
			wantUnrefinedCount: 1,
			wantUnplannedCount: 0,
			wantUndecomposed:   0,
			wantRecommendation: "Refine idea: Implement dark mode",
		},
		{
			name: "specs without plans",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				specsDir := filepath.Join(gromitDir, "specs")
				plansDir := filepath.Join(gromitDir, "plans")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(specsDir, 0755)
				os.MkdirAll(plansDir, 0755)

				// Create spec without plan
				os.WriteFile(filepath.Join(specsDir, "user-auth.md"), []byte("# User Auth Spec"), 0644)
				os.WriteFile(filepath.Join(specsDir, "dark-mode.md"), []byte("# Dark Mode Spec"), 0644)

				// Create plan for one spec
				os.WriteFile(filepath.Join(plansDir, "dark-mode.md"), []byte("---\ndecomposed: false\n---\n# Dark Mode Plan"), 0644)
			},
			wantUnrefinedCount: 0,
			wantUnplannedCount: 1,
			wantUndecomposed:   1,
			wantRecommendation: "Plan spec \"user-auth\"",
		},
		{
			name: "undecomposed plans",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				specsDir := filepath.Join(gromitDir, "specs")
				plansDir := filepath.Join(gromitDir, "plans")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(specsDir, 0755)
				os.MkdirAll(plansDir, 0755)

				// Create plans with different decomposed states
				os.WriteFile(filepath.Join(plansDir, "user-auth.md"), []byte("---\ndecomposed: false\n---\n\n# Plan"), 0644)
				os.WriteFile(filepath.Join(plansDir, "dark-mode.md"), []byte("---\ndecomposed: true\n---\n\n# Plan"), 0644)
				os.WriteFile(filepath.Join(plansDir, "api.md"), []byte("# Plan with no decomposed field"), 0644)
			},
			wantUnrefinedCount: 0,
			wantUnplannedCount: 0,
			wantUndecomposed:   2,                        // user-auth (false) and api (missing)
			wantRecommendation: "Decompose plan \"api\"", // api comes first alphabetically
		},
		{
			name: "decomposed plans not counted",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				specsDir := filepath.Join(gromitDir, "specs")
				plansDir := filepath.Join(gromitDir, "plans")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(specsDir, 0755)
				os.MkdirAll(plansDir, 0755)

				os.WriteFile(filepath.Join(plansDir, "user-auth.md"), []byte("---\ndecomposed: true\n---\n# Plan"), 0644)
			},
			wantUnrefinedCount: 0,
			wantUnplannedCount: 0,
			wantUndecomposed:   0,
			wantRecommendation: "No work in pipeline",
		},
		{
			name: "priority unrefined over unplanned",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				specsDir := filepath.Join(gromitDir, "specs")
				plansDir := filepath.Join(gromitDir, "plans")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(specsDir, 0755)
				os.MkdirAll(plansDir, 0755)

				// Add unrefined idea
				bf, _ := backlog.NewFile(gromitDir)
				bf.Add(&backlog.Idea{
					ID:     "idea-1",
					Text:   "New feature",
					Status: "",
				})

				// Add unplanned spec
				os.WriteFile(filepath.Join(specsDir, "user-auth.md"), []byte("# User Auth Spec"), 0644)
			},
			wantUnrefinedCount: 1,
			wantUnplannedCount: 1,
			wantUndecomposed:   0,
			wantRecommendation: "Refine idea: New feature",
		},
		{
			name: "priority unplanned over undecomposed",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				specsDir := filepath.Join(gromitDir, "specs")
				plansDir := filepath.Join(gromitDir, "plans")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(specsDir, 0755)
				os.MkdirAll(plansDir, 0755)

				// Add unplanned spec
				os.WriteFile(filepath.Join(specsDir, "user-auth.md"), []byte("# User Auth Spec"), 0644)

				// Add undecomposed plan
				os.WriteFile(filepath.Join(plansDir, "dark-mode.md"), []byte("---\ndecomposed: false\n---\n# Plan"), 0644)
			},
			wantUnrefinedCount: 0,
			wantUnplannedCount: 1,
			wantUndecomposed:   1,
			wantRecommendation: "Plan spec \"user-auth\"",
		},
		{
			name: "long idea text is truncated in recommendation",
			setupFunc: func(t *testing.T, tmpDir string) {
				gromitDir := filepath.Join(tmpDir, ".gromit")
				os.MkdirAll(gromitDir, 0755)
				os.MkdirAll(filepath.Join(gromitDir, "specs"), 0755)
				os.MkdirAll(filepath.Join(gromitDir, "plans"), 0755)

				bf, _ := backlog.NewFile(gromitDir)
				bf.Add(&backlog.Idea{
					ID:     "idea-1",
					Text:   "This is a very long idea text that should be truncated in the recommendation to avoid cluttering the display",
					Status: "",
				})
			},
			wantUnrefinedCount: 1,
			wantUnplannedCount: 0,
			wantUndecomposed:   0,
			wantRecommendation: "Refine idea: This is a very long idea text that should be tr...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			gromitDir := filepath.Join(tmpDir, ".gromit")
			specsDir := filepath.Join(gromitDir, "specs")
			plansDir := filepath.Join(gromitDir, "plans")

			// Run setup
			tt.setupFunc(t, tmpDir)

			// Read status
			status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
			if err != nil {
				t.Fatalf("ReadStatus() error = %v", err)
			}

			// Check counts
			if status.UnrefinedCount != tt.wantUnrefinedCount {
				t.Errorf("UnrefinedCount = %d, want %d", status.UnrefinedCount, tt.wantUnrefinedCount)
			}

			if len(status.UnplannedSpecs) != tt.wantUnplannedCount {
				t.Errorf("UnplannedSpecs count = %d, want %d", len(status.UnplannedSpecs), tt.wantUnplannedCount)
			}

			if len(status.UndecomposedPlans) != tt.wantUndecomposed {
				t.Errorf("UndecomposedPlans count = %d, want %d", len(status.UndecomposedPlans), tt.wantUndecomposed)
			}

			// Check recommendation
			if status.Recommendation != tt.wantRecommendation {
				t.Errorf("Recommendation = %q, want %q", status.Recommendation, tt.wantRecommendation)
			}
		})
	}
}

func TestReadStatus_MissingDirectories(t *testing.T) {
	disableLiveBDForStatusTests(t)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	// Don't create any directories - ReadStatus should handle this gracefully
	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() with missing directories should not error, got: %v", err)
	}

	if status.UnrefinedCount != 0 {
		t.Errorf("UnrefinedCount = %d, want 0", status.UnrefinedCount)
	}

	if len(status.UnplannedSpecs) != 0 {
		t.Errorf("UnplannedSpecs count = %d, want 0", len(status.UnplannedSpecs))
	}

	if len(status.UndecomposedPlans) != 0 {
		t.Errorf("UndecomposedPlans count = %d, want 0", len(status.UndecomposedPlans))
	}

	if status.Recommendation != "No work in pipeline" {
		t.Errorf("Recommendation = %q, want %q", status.Recommendation, "No work in pipeline")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string not truncated",
			input:  "Hello",
			maxLen: 10,
			want:   "Hello",
		},
		{
			name:   "exact length not truncated",
			input:  "HelloWorld",
			maxLen: 10,
			want:   "HelloWorld",
		},
		{
			name:   "long string truncated with ellipsis",
			input:  "This is a very long string",
			maxLen: 10,
			want:   "This is...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "maxLen less than 3",
			input:  "Hello world this is a long string",
			maxLen: 1,
			want:   "...",
		},
		{
			name:   "maxLen equals 3",
			input:  "Hello world",
			maxLen: 3,
			want:   "...",
		},
		{
			name:   "maxLen equals 4",
			input:  "Hello world",
			maxLen: 4,
			want:   "H...",
		},
		{
			name:   "UTF-8 multibyte characters",
			input:  "こんにちは世界",
			maxLen: 10,
			want:   "こんにちは世界",
		},
		{
			name:   "UTF-8 multibyte characters truncated",
			input:  "こんにちは世界はとても長いです",
			maxLen: 5,
			want:   "こん...",
		},
		{
			name:   "mixed ASCII and UTF-8",
			input:  "Hello こんにちは World",
			maxLen: 10,
			want:   "Hello こ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestGenerateRecommendation(t *testing.T) {
	tests := []struct {
		name   string
		status *PipelineStatus
		want   string
	}{
		{
			name: "empty pipeline",
			status: &PipelineStatus{
				UnrefinedCount:    0,
				UnrefinedIdeas:    []string{},
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    0,
			},
			want: "No work in pipeline",
		},
		{
			name: "unrefined ideas with text",
			status: &PipelineStatus{
				UnrefinedCount: 2,
				UnrefinedIdeas: []string{"Add login", "Add signup"},
			},
			want: "Refine idea: Add login",
		},
		{
			name: "unrefined ideas without text",
			status: &PipelineStatus{
				UnrefinedCount: 2,
				UnrefinedIdeas: []string{},
			},
			want: "Refine backlog ideas",
		},
		{
			name: "unplanned specs",
			status: &PipelineStatus{
				UnrefinedCount: 0,
				UnplannedSpecs: []string{"user-auth", "dark-mode"},
			},
			want: "Plan spec \"user-auth\"",
		},
		{
			name: "undecomposed plans",
			status: &PipelineStatus{
				UnrefinedCount:    0,
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{"api-endpoints"},
			},
			want: "Decompose plan \"api-endpoints\"",
		},
		{
			name: "ready beads only",
			status: &PipelineStatus{
				UnrefinedCount:    0,
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    4,
			},
			want: "Run 4 ready bead(s)",
		},
		{
			name: "ready beads singular",
			status: &PipelineStatus{
				UnrefinedCount:    0,
				UnplannedSpecs:    []string{},
				UndecomposedPlans: []string{},
				ReadyBeadCount:    1,
			},
			want: "Run 1 ready bead(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRecommendation(tt.status)
			if got != tt.want {
				t.Errorf("generateRecommendation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadStatus_ErrorHandling(t *testing.T) {
	disableLiveBDForStatusTests(t)

	t.Run("corrupt backlog file", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// Write corrupt JSON to backlog
		backlogPath := filepath.Join(gromitDir, "backlog.jsonl")
		os.WriteFile(backlogPath, []byte("{invalid json\n"), 0644)

		// ReadStatus should return an error
		_, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err == nil {
			t.Fatal("expected error with corrupt backlog file, got nil")
		}
	})

	t.Run("corrupt plan frontmatter", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// Create plan with malformed frontmatter
		planPath := filepath.Join(plansDir, "bad-plan.md")
		os.WriteFile(planPath, []byte("---\nthis is not valid yaml: {[\n---\n# Plan"), 0644)

		// ReadStatus should return an error
		_, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err == nil {
			t.Fatal("expected error with corrupt plan frontmatter, got nil")
		}
	})

	t.Run("permission error on specs directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0000) // no permissions
		os.MkdirAll(plansDir, 0755)

		// ReadStatus should return an error
		_, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err == nil {
			// Clean up permission issue before test framework tries to delete it
			os.Chmod(specsDir, 0755)
			t.Fatal("expected error with unreadable specs directory, got nil")
		}

		// Clean up permission issue
		os.Chmod(specsDir, 0755)
	})

	t.Run("permission error on plans directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0000) // no permissions

		// ReadStatus should return an error
		_, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err == nil {
			// Clean up permission issue before test framework tries to delete it
			os.Chmod(plansDir, 0755)
			t.Fatal("expected error with unreadable plans directory, got nil")
		}

		// Clean up permission issue
		os.Chmod(plansDir, 0755)
	})
}

func TestReadStatus_CountingAccuracy(t *testing.T) {
	disableLiveBDForStatusTests(t)

	t.Run("multiple items in each category", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// Add multiple unrefined ideas
		bf, _ := backlog.NewFile(gromitDir)
		bf.Add(&backlog.Idea{ID: "idea-1", Text: "Idea one", Status: ""})
		bf.Add(&backlog.Idea{ID: "idea-2", Text: "Idea two", Status: ""})
		bf.Add(&backlog.Idea{ID: "idea-3", Text: "Idea three", Status: "refined"})
		bf.Add(&backlog.Idea{ID: "idea-4", Text: "Idea four", Status: ""})

		// Add multiple specs (some with plans, some without)
		os.WriteFile(filepath.Join(specsDir, "spec-a.md"), []byte("# Spec A"), 0644)
		os.WriteFile(filepath.Join(specsDir, "spec-b.md"), []byte("# Spec B"), 0644)
		os.WriteFile(filepath.Join(specsDir, "spec-c.md"), []byte("# Spec C"), 0644)

		// Create plans for some specs
		os.WriteFile(filepath.Join(plansDir, "spec-a.md"), []byte("---\ndecomposed: false\n---\n# Plan A"), 0644)
		os.WriteFile(filepath.Join(plansDir, "spec-b.md"), []byte("---\ndecomposed: true\n---\n# Plan B"), 0644)
		// spec-c has no plan

		// spec-a should be counted as undecomposed
		// spec-b should not be counted (decomposed: true)
		// spec-c should be counted as unplanned

		status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err != nil {
			t.Fatalf("ReadStatus() error = %v", err)
		}

		if status.UnrefinedCount != 3 {
			t.Errorf("UnrefinedCount = %d, want 3", status.UnrefinedCount)
		}

		if len(status.UnrefinedIdeas) != 3 {
			t.Errorf("len(UnrefinedIdeas) = %d, want 3", len(status.UnrefinedIdeas))
		}

		if len(status.UnplannedSpecs) != 1 {
			t.Errorf("len(UnplannedSpecs) = %d, want 1 (spec-c)", len(status.UnplannedSpecs))
		}

		if len(status.UndecomposedPlans) != 1 {
			t.Errorf("len(UndecomposedPlans) = %d, want 1 (spec-a)", len(status.UndecomposedPlans))
		}
	})

	t.Run("empty backlog with work in other stages", func(t *testing.T) {
		tmpDir := t.TempDir()
		gromitDir := filepath.Join(tmpDir, ".gromit")
		specsDir := filepath.Join(gromitDir, "specs")
		plansDir := filepath.Join(gromitDir, "plans")

		os.MkdirAll(gromitDir, 0755)
		os.MkdirAll(specsDir, 0755)
		os.MkdirAll(plansDir, 0755)

		// No backlog items
		// Add specs and plans
		os.WriteFile(filepath.Join(specsDir, "feature.md"), []byte("# Feature"), 0644)
		os.WriteFile(filepath.Join(plansDir, "api.md"), []byte("---\ndecomposed: false\n---\n# API Plan"), 0644)

		status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
		if err != nil {
			t.Fatalf("ReadStatus() error = %v", err)
		}

		if status.UnrefinedCount != 0 {
			t.Errorf("UnrefinedCount = %d, want 0", status.UnrefinedCount)
		}

		if len(status.UnplannedSpecs) != 1 {
			t.Errorf("len(UnplannedSpecs) = %d, want 1", len(status.UnplannedSpecs))
		}

		if len(status.UndecomposedPlans) != 1 {
			t.Errorf("len(UndecomposedPlans) = %d, want 1", len(status.UndecomposedPlans))
		}
	})
}

func TestReadStatus_SpecNamesSorted(t *testing.T) {
	disableLiveBDForStatusTests(t)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Create specs in non-alphabetical order
	os.WriteFile(filepath.Join(specsDir, "zebra.md"), []byte("# Zebra"), 0644)
	os.WriteFile(filepath.Join(specsDir, "apple.md"), []byte("# Apple"), 0644)
	os.WriteFile(filepath.Join(specsDir, "mango.md"), []byte("# Mango"), 0644)

	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Check that specs are sorted
	expected := []string{"apple", "mango", "zebra"}
	if len(status.UnplannedSpecs) != len(expected) {
		t.Fatalf("len(UnplannedSpecs) = %d, want %d", len(status.UnplannedSpecs), len(expected))
	}

	for i, want := range expected {
		if status.UnplannedSpecs[i] != want {
			t.Errorf("UnplannedSpecs[%d] = %q, want %q", i, status.UnplannedSpecs[i], want)
		}
	}
}

func TestReadStatus_PlanNamesSorted(t *testing.T) {
	disableLiveBDForStatusTests(t)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Create plans in non-alphabetical order (all undecomposed)
	os.WriteFile(filepath.Join(plansDir, "zebra.md"), []byte("---\ndecomposed: false\n---\n# Zebra"), 0644)
	os.WriteFile(filepath.Join(plansDir, "apple.md"), []byte("# Apple (no frontmatter)"), 0644)
	os.WriteFile(filepath.Join(plansDir, "mango.md"), []byte("---\ndecomposed: false\n---\n# Mango"), 0644)

	status, err := ReadStatus(gromitDir, specsDir, plansDir, nil)
	if err != nil {
		t.Fatalf("ReadStatus() error = %v", err)
	}

	// Check that plans are sorted
	expected := []string{"apple", "mango", "zebra"}
	if len(status.UndecomposedPlans) != len(expected) {
		t.Fatalf("len(UndecomposedPlans) = %d, want %d", len(status.UndecomposedPlans), len(expected))
	}

	for i, want := range expected {
		if status.UndecomposedPlans[i] != want {
			t.Errorf("UndecomposedPlans[%d] = %q, want %q", i, status.UndecomposedPlans[i], want)
		}
	}
}

// testBacklogClientWithIdeas implements BacklogClient with preset ideas
type testBacklogClientWithIdeas struct {
	ideas []*Idea
}

func (m *testBacklogClientWithIdeas) List() ([]*Idea, error) {
	return m.ideas, nil
}

func (m *testBacklogClientWithIdeas) Get(id string) (*Idea, error) {
	for _, idea := range m.ideas {
		if idea.ID == id {
			return idea, nil
		}
	}
	return nil, nil
}

func (m *testBacklogClientWithIdeas) Add(item *Idea) error {
	m.ideas = append(m.ideas, item)
	return nil
}

func (m *testBacklogClientWithIdeas) Update(id string, fn func(*Idea)) error {
	for _, idea := range m.ideas {
		if idea.ID == id {
			fn(idea)
			return nil
		}
	}
	return nil
}

func TestReadStatus_WithInjectedDependencies(t *testing.T) {
	disableLiveBDForStatusTests(t)

	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(plansDir, 0755)

	// Create a mock BacklogClient with test data
	mockBacklog := &testBacklogClientWithIdeas{
		ideas: []*Idea{
			{
				ID:     "idea-1",
				Text:   "Test idea",
				Status: "",
			},
		},
	}

	// Create a mock BeadQueryClient
	mockBeadQueryClient := &testBeadQueryClientStatus{
		readyBeads: []string{},
	}

	// This should work with dependency injection
	status, err := ReadStatusWithDeps(gromitDir, specsDir, plansDir, nil, mockBacklog, mockBeadQueryClient)
	if err != nil {
		t.Fatalf("ReadStatusWithDeps() error = %v", err)
	}

	if status.UnrefinedCount != 1 {
		t.Errorf("UnrefinedCount = %d, want 1", status.UnrefinedCount)
	}
}
