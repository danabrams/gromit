package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvResolver_GROMIT_HOME(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GROMIT_HOME", dir)
	t.Setenv("XDG_DATA_HOME", "")

	r := NewEnvResolver()
	root, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if string(root) != dir {
		t.Errorf("root = %q, want %q", root, dir)
	}
}

func TestEnvResolver_XDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GROMIT_HOME", "")
	t.Setenv("XDG_DATA_HOME", dir)

	r := NewEnvResolver()
	root, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(dir, "gromit")
	if string(root) != want {
		t.Errorf("root = %q, want %q", root, want)
	}
}

func TestEnvResolver_Default(t *testing.T) {
	t.Setenv("GROMIT_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	home, _ := os.UserHomeDir()

	r := NewEnvResolver()
	root, err := r.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := filepath.Join(home, ".local", "share", "gromit")
	if string(root) != want {
		t.Errorf("root = %q, want %q", root, want)
	}
}

func TestRoot_ProjectsDir(t *testing.T) {
	root := Root("/workspace")
	if got := root.ProjectsDir(); got != "/workspace/projects" {
		t.Errorf("ProjectsDir = %q, want %q", got, "/workspace/projects")
	}
}

func TestRoot_ProjectCell(t *testing.T) {
	root := Root("/workspace")
	if got := root.ProjectCell("myapp"); got != "/workspace/projects/myapp" {
		t.Errorf("ProjectCell = %q, want %q", got, "/workspace/projects/myapp")
	}
}
