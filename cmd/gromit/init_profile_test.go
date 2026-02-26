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
	cfg := string(content)
	if !strings.Contains(cfg, "project:\n  profile: \"node\"") {
		t.Fatalf("gromit.yaml missing profile header, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "- \"npm test\"") {
		t.Fatalf("gromit.yaml missing node validation command, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "- \"npm run build\"") {
		t.Fatalf("gromit.yaml missing node validation build command, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "compile_command: \"npm run build\"") {
		t.Fatalf("gromit.yaml missing node compile command, got:\n%s", cfg)
	}
}

func TestInitRespectsProfileFlag(t *testing.T) {
	dir := t.TempDir()
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	defer os.Chdir(prevWd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := os.WriteFile("go.mod", []byte("module example"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	prevForce := forceInit
	prevProfile := initProfile
	defer func() {
		forceInit = prevForce
		initProfile = prevProfile
	}()
	forceInit = true
	initProfile = "python"

	if err := runInit(nil, nil); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "gromit.yaml"))
	if err != nil {
		t.Fatalf("read gromit.yaml: %v", err)
	}
	cfg := string(content)
	if !strings.Contains(cfg, "project:\n  profile: \"python\"") {
		t.Fatalf("gromit.yaml missing python profile header, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "- \"pytest\"") {
		t.Fatalf("gromit.yaml missing python validation command, got:\n%s", cfg)
	}
	if !strings.Contains(cfg, "compile_command: \"\"") {
		t.Fatalf("gromit.yaml missing python compile command override, got:\n%s", cfg)
	}
}

func TestExplicitProfileOverrideNoImplicitInjection(t *testing.T) {
	testCases := []struct {
		name             string
		explicitProfile  string
		shouldHaveTracker bool
		shouldHaveAdapter bool
	}{
		{
			name:             "explicit profile does not implicitly set tracker backend",
			explicitProfile:  "go",
			shouldHaveTracker: false,
			shouldHaveAdapter: false,
		},
		{
			name:             "explicit python profile override ignores go.mod signal",
			explicitProfile:  "python",
			shouldHaveTracker: false,
			shouldHaveAdapter: false,
		},
		{
			name:             "explicit node profile override ignores pyproject.toml signal",
			explicitProfile:  "node",
			shouldHaveTracker: false,
			shouldHaveAdapter: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			// Create multiple signal files to ensure we're using explicit override
			if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
				t.Fatalf("write go.mod: %v", err)
			}
			if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[build-system]"), 0644); err != nil {
				t.Fatalf("write pyproject.toml: %v", err)
			}

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
			initProfile = tc.explicitProfile

			if err := runInit(nil, nil); err != nil {
				t.Fatalf("runInit: %v", err)
			}

			content, err := os.ReadFile(filepath.Join(dir, "gromit.yaml"))
			if err != nil {
				t.Fatalf("read gromit.yaml: %v", err)
			}
			cfg := string(content)

			// Verify profile is explicit
			if !strings.Contains(cfg, "project:\n  profile: \""+tc.explicitProfile+"\"") {
				t.Fatalf("gromit.yaml missing expected profile, got:\n%s", cfg)
			}

			// Verify NO implicit tracker backend injection
			if tc.shouldHaveTracker {
				if !strings.Contains(cfg, "tracker:") {
					t.Errorf("expected tracker section in config, got:\n%s", cfg)
				}
			} else {
				if strings.Contains(cfg, "  backend:") && strings.Contains(cfg, "tracker:") {
					t.Errorf("unexpected tracker.backend section in config (explicit profile should not implicitly inject tracker), got:\n%s", cfg)
				}
			}

			// Verify NO implicit methodology adapter injection
			if tc.shouldHaveAdapter {
				if !strings.Contains(cfg, "methodology:") {
					t.Errorf("expected methodology section in config, got:\n%s", cfg)
				}
			}
		})
	}
}

func TestSelectInitProfilePrecedenceExplicitOverDetection(t *testing.T) {
	dir := t.TempDir()

	// Create go.mod to signal "go" profile
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	// Create package.json to signal "node" profile
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	prevProfile := initProfile
	defer func() { initProfile = prevProfile }()

	// Set explicit profile
	initProfile = "python"

	profile, err := selectInitProfile(dir)
	if err != nil {
		t.Fatalf("selectInitProfile: %v", err)
	}

	// Explicit profile should take precedence over detected signals
	if profile != "python" {
		t.Fatalf("selectInitProfile() = %q, want python (explicit should override detection)", profile)
	}
}

func TestSelectInitProfileCustomProfile(t *testing.T) {
	dir := t.TempDir()

	prevProfile := initProfile
	defer func() { initProfile = prevProfile }()

	initProfile = "custom"

	profile, err := selectInitProfile(dir)
	if err != nil {
		t.Fatalf("selectInitProfile: %v", err)
	}

	if profile != "custom" {
		t.Fatalf("selectInitProfile() = %q, want custom", profile)
	}
}
