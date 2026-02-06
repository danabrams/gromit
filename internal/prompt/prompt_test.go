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
