package config

import (
	"reflect"
	"testing"
)

func TestProfileForName(t *testing.T) {
	testCases := []struct {
		name         string
		wantCommands []string
		wantFound    bool
	}{
		{
			name:         "go",
			wantCommands: []string{"go test", "go build", "go vet"},
			wantFound:    true,
		},
		{
			name:         "node",
			wantCommands: []string{"npm test", "npm run build"},
			wantFound:    true,
		},
		{
			name:         "python",
			wantCommands: []string{"pytest"},
			wantFound:    true,
		},
		{
			name:      "custom",
			wantFound: true,
		},
		{
			name:      "unknown",
			wantFound: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaults, ok := ProfileForName(tc.name)
			if ok != tc.wantFound {
				t.Fatalf("found = %v, want %v", ok, tc.wantFound)
			}
			if !tc.wantFound {
				return
			}
			if !reflect.DeepEqual(defaults.ValidationCommands, tc.wantCommands) {
				t.Fatalf("ValidationCommands = %v, want %v", defaults.ValidationCommands, tc.wantCommands)
			}
		})
	}
}

func TestEffectiveValidationCommandsExplicitPrecedence(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
		Validation: ValidationConfig{
			Commands: []string{"custom test", "custom build"},
		},
	}

	effective := cfg.EffectiveValidationCommands()

	// Explicit commands take precedence over profile defaults
	want := []string{"custom test", "custom build"}
	if !reflect.DeepEqual(effective, want) {
		t.Errorf("EffectiveValidationCommands() = %v, want %v", effective, want)
	}
}

func TestEffectiveValidationCommandsProfileDefaults(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
		Validation: ValidationConfig{
			Commands: nil, // No explicit commands
		},
	}

	effective := cfg.EffectiveValidationCommands()

	// Profile defaults should be used when explicit commands are empty
	want := []string{"go test", "go build", "go vet"}
	if !reflect.DeepEqual(effective, want) {
		t.Errorf("EffectiveValidationCommands() = %v, want %v", effective, want)
	}
}

func TestEffectivePreflightCompileCommandExplicitPrecedence(t *testing.T) {
	cfg := Config{
		Project: ProjectConfig{
			Profile: "go",
		},
		Preflight: PreflightConfig{
			CompileCommand: "custom compile",
		},
	}

	effective := cfg.EffectivePreflightCompileCommand()

	// Explicit compile command takes precedence over profile defaults
	want := "custom compile"
	if effective != want {
		t.Errorf("EffectivePreflightCompileCommand() = %q, want %q", effective, want)
	}
}
