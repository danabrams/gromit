package config

import (
	"testing"
)

func TestSessionConfigDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	// Iterations default: 0 means unlimited, so no default to set
	// TestCommand default: empty string
	if cfg.Session.TestCommand != "" {
		t.Errorf("Session.TestCommand default = %q, want empty string", cfg.Session.TestCommand)
	}

	// MaxFixRetries default: 3
	if cfg.Session.MaxFixRetries != 3 {
		t.Errorf("Session.MaxFixRetries default = %d, want 3", cfg.Session.MaxFixRetries)
	}

	// FixTier default: "medium"
	if cfg.Session.FixTier != "medium" {
		t.Errorf("Session.FixTier default = %q, want %q", cfg.Session.FixTier, "medium")
	}

	// Review default: true
	if cfg.Session.Review == nil {
		t.Error("Session.Review default = nil, want non-nil (true)")
	} else if !*cfg.Session.Review {
		t.Errorf("Session.Review default = false, want true")
	}

	// Retro default: true
	if cfg.Session.Retro == nil {
		t.Error("Session.Retro default = nil, want non-nil (true)")
	} else if !*cfg.Session.Retro {
		t.Errorf("Session.Retro default = false, want true")
	}
}
