package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectProfilePriority(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{name: "go takes precedence", files: []string{"go.mod", "package.json"}, want: "go"},
		{name: "node before python", files: []string{"package.json", "pyproject.toml"}, want: "node"},
		{name: "python when only python signal", files: []string{"pyproject.toml"}, want: "python"},
		{name: "go when no signal", files: nil, want: "go"},
	}

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            dir := t.TempDir()
            for _, file := range tc.files {
                path := filepath.Join(dir, file)
                if err := os.WriteFile(path, []byte(""), 0644); err != nil {
                    t.Fatalf("write signal file %s: %v", file, err)
                }
            }

            if got := detectProfile(dir); got != tc.want {
                t.Fatalf("detectProfile() = %q, want %q", got, tc.want)
            }
        })
    }
}

func TestDetectProfileFallbacksToGo(t *testing.T) {
	dir := t.TempDir()
	if got := detectProfile(dir); got != "go" {
		t.Fatalf("detectProfile() = %q, want go when no signals", got)
	}
}

func TestSelectProfilePrefersExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte{}, 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	prevProfile := initProfile
	initProfile = "python"
	defer func() {
		initProfile = prevProfile
	}()

	profile, err := selectInitProfile(dir)
	if err != nil {
		t.Fatalf("selectInitProfile: %v", err)
	}
	if profile != "python" {
		t.Fatalf("selectInitProfile() = %q, want python", profile)
	}
}

func TestSelectProfileUsesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	content := `project:
  profile: "node"
`
	if err := os.WriteFile(filepath.Join(dir, "gromit.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write gromit.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	profile, err := selectInitProfile(dir)
	if err != nil {
		t.Fatalf("selectInitProfile: %v", err)
	}
	if profile != "node" {
		t.Fatalf("selectInitProfile() = %q, want node", profile)
	}
}

func TestInitWritesDetectedProfile(t *testing.T) {
	setupDir := func(t *testing.T) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
			t.Fatalf("write package.json: %v", err)
		}
		return dir
	}

	dir := setupDir(t)
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	defer os.Chdir(prevWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	prevForce := forceInit
	prevProfile := initProfile
	defer func() {
		forceInit = prevForce
		initProfile = prevProfile
	}()
	forceInit = true
	initProfile = ""

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "gromit.yaml"))
	if err != nil {
		t.Fatalf("read gromit.yaml: %v", err)
	}
	if !strings.Contains(string(content), "project:\n  profile: \"node\"") {
		t.Fatalf("gromit.yaml missing profile header, got:\n%s", string(content))
	}
}
