package config

import "testing"

func TestSupplementaryConfigFilesAreGofmted(t *testing.T) {
	if err := EnsureSupplementaryConfigFilesFormatted(); err != nil {
		t.Fatalf("supplementary config files must be gofmt-compliant: %v", err)
	}
}
