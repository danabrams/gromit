package config

import "testing"

func TestSupplementaryConfigFilesAreGofmted(t *testing.T) {
	if err := EnsureSupplementaryConfigFilesFormatted(); err != nil {
		t.Fatalf("supplementary config files must be gofmt-compliant: %v", err)
	}
}

func TestSupplementaryConfigFilesIncludeCoreConfigHelpers(t *testing.T) {
	required := []string{
		"config_normalize.go",
		"config_defaults.go",
		"config_accessors.go",
	}
	for _, file := range required {
		found := false
		for _, candidate := range supplementaryConfigFiles {
			if candidate == file {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("supplementary config gofmt list missing %s", file)
		}
	}
}
