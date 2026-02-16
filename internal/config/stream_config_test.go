package config

import (
	"os"
	"testing"
)

func TestSetDefaults_StreamPreserveProviderOutputDefaultsTrue(t *testing.T) {
	var cfg Config
	cfg.SetDefaults()

	if cfg.Stream.PreserveProviderOutput == nil {
		t.Fatal("Stream.PreserveProviderOutput is nil after SetDefaults")
	}
	if !cfg.Stream.PreserveProviderOutputEnabled() {
		t.Fatal("Stream.PreserveProviderOutputEnabled() = false, want true")
	}
}

func TestLoad_StreamPreserveProviderOutputFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/gromit.yaml"
	data := []byte("stream:\n  preserve_provider_output: false\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Stream.PreserveProviderOutputEnabled() {
		t.Fatal("Stream.PreserveProviderOutputEnabled() = true, want false")
	}
}
