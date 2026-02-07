package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/backlog"
)

func TestReadStatus(t *testing.T) {
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
			status, err := ReadStatus(gromitDir, specsDir, plansDir)
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
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	plansDir := filepath.Join(gromitDir, "plans")

	// Don't create any directories - ReadStatus should handle this gracefully
	status, err := ReadStatus(gromitDir, specsDir, plansDir)
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
