package runner

import (
	"testing"

	"github.com/danabrams/gromit/internal/config"
)

func TestRefactorConfigDefaultMinFilesChanged(t *testing.T) {
	cfg := &config.Config{}
	cfg.SetDefaults()

	// Verify default is 3
	if cfg.Refactor.MinFilesChanged != 3 {
		t.Errorf("Default MinFilesChanged = %d, want 3", cfg.Refactor.MinFilesChanged)
	}
}
