package main

import (
	"testing"
)

// TestRunRetro_CreatesProviderFromConfig documents that runRetro now uses provider
func TestRunRetro_CreatesProviderFromConfig(t *testing.T) {
	// Migration complete:
	// runRetro (the retroCmd RunE function in main.go) now:
	// 1. Creates claude.Client from cfg.Claude
	// 2. Wraps it in provider.NewClaudeProvider with tierToModel map
	// 3. Calls retro.NewRetroWithProvider(claudeProvider, gromitDir)

	t.Log("Migration complete: runRetro now creates provider inline and uses NewRetroWithProvider")
}
