package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateSpecName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "auth", false},
		{"valid hyphenated name", "user-auth", false},
		{"valid underscore name", "user_auth", false},
		{"empty name", "", true},
		{"dot-dot traversal", "../../etc/passwd", true},
		{"single dot-dot", "..", true},
		{"dot-dot in middle", "foo/../bar", true},
		{"forward slash", "foo/bar", true},
		{"backslash", "foo\\bar", true},
		{"absolute path attempt", "/etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpecName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSpecName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestLoadSpec_PathTraversal(t *testing.T) {
	// Create a temp directory structure
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	os.MkdirAll(specsDir, 0755)

	// Create a valid spec file
	os.WriteFile(filepath.Join(specsDir, "valid.md"), []byte("# Valid Spec"), 0644)

	// Create a secret file outside specs dir
	os.WriteFile(filepath.Join(tmpDir, "secret.md"), []byte("SECRET DATA"), 0644)

	r := &Renderer{specsDir: specsDir}

	// Valid spec should load fine
	content, err := r.LoadSpec("valid")
	if err != nil {
		t.Fatalf("LoadSpec(valid) unexpected error: %v", err)
	}
	if content != "# Valid Spec" {
		t.Errorf("LoadSpec(valid) = %q, want %q", content, "# Valid Spec")
	}

	// Path traversal attempts should be rejected
	traversalNames := []string{
		"../secret",
		"../../etc/passwd",
		"..",
		"foo/../../../secret",
		"foo/bar",
		"foo\\bar",
	}
	for _, name := range traversalNames {
		content, err := r.LoadSpec(name)
		if err == nil {
			t.Errorf("LoadSpec(%q) should have returned error, got content: %q", name, content)
		}
	}

	// Missing spec should return empty string, no error
	content, err = r.LoadSpec("nonexistent")
	if err != nil {
		t.Errorf("LoadSpec(nonexistent) unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("LoadSpec(nonexistent) = %q, want empty", content)
	}
}

func TestGetLearningsFileNilReceiver(t *testing.T) {
	var r *Renderer
	result := r.GetLearningsFile()
	if result != nil {
		t.Errorf("expected nil for nil Renderer, got %v", result)
	}
}

func TestBuildContextNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.BuildContext(nil, nil, 1, "sonnet")
	if err == nil {
		t.Error("expected error for nil renderer")
	}
}

func TestBuildContextNilBead(t *testing.T) {
	r := &Renderer{}
	_, err := r.BuildContext(nil, nil, 1, "sonnet")
	if err == nil {
		t.Error("expected error for nil bead")
	}
}

func TestRenderNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderBuild(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderBuild")
	}
	_, err = r.RenderAnalyze(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderAnalyze")
	}
}

func TestLoadClaudeMDNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadClaudeMD()
	if err == nil {
		t.Error("expected error for nil renderer in LoadClaudeMD")
	}
}

func TestLoadRulesNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadRules()
	if err == nil {
		t.Error("expected error for nil renderer in LoadRules")
	}
}

func TestLoadSpecNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.LoadSpec("test")
	if err == nil {
		t.Error("expected error for nil renderer in LoadSpec")
	}
}

func TestRenderDecomposeNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderDecompose(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderDecompose")
	}
}

func TestRenderScopeNilRenderer(t *testing.T) {
	var r *Renderer
	_, err := r.RenderScope(nil)
	if err == nil {
		t.Error("expected error for nil renderer in RenderScope")
	}
}

func TestParseScopeEstimate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		checks  func(*ScopeEstimate) bool
	}{
		{
			name: "valid scope estimate",
			input: `{
  "complexity": "medium",
  "estimated_iterations": 2,
  "rationale": "Changes to 3-5 files with moderate testing",
  "can_complete_in_single_iteration": false,
  "blockers": ["Need to refactor auth system", "Database schema changes required"]
}`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "medium" &&
					est.EstimatedIterations == 2 &&
					est.Rationale == "Changes to 3-5 files with moderate testing" &&
					!est.CanCompleteInSingleIteration &&
					len(est.Blockers) == 2
			},
		},
		{
			name: "low complexity single iteration",
			input: `{
  "complexity": "low",
  "estimated_iterations": 1,
  "rationale": "Straightforward changes",
  "can_complete_in_single_iteration": true,
  "blockers": []
}`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "low" &&
					est.EstimatedIterations == 1 &&
					est.CanCompleteInSingleIteration &&
					len(est.Blockers) == 0
			},
		},
		{
			name: "high complexity with multiple blockers",
			input: `{
  "complexity": "high",
  "estimated_iterations": 4,
  "rationale": "Complex architecture changes across multiple systems",
  "can_complete_in_single_iteration": false,
  "blockers": ["Unclear requirements", "Cross-system dependencies", "Performance implications"]
}`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "high" &&
					est.EstimatedIterations == 4 &&
					len(est.Blockers) == 3
			},
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no JSON in output",
			input:   "This is just plain text with no JSON",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `{ "complexity": "low", invalid json }`,
			wantErr: true,
		},
		{
			name: "JSON with surrounding text",
			input: `Here is the scope estimate:
{
  "complexity": "low",
  "estimated_iterations": 1,
  "rationale": "Simple task",
  "can_complete_in_single_iteration": true,
  "blockers": []
}
Additional explanation follows...`,
			wantErr: false,
			checks: func(est *ScopeEstimate) bool {
				return est.Complexity == "low" &&
					est.EstimatedIterations == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			est, err := ParseScopeEstimate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseScopeEstimate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checks != nil {
				if !tt.checks(est) {
					t.Errorf("ParseScopeEstimate() returned unexpected values: %+v", est)
				}
			}
		})
	}
}
