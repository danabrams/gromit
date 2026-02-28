package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveExploreModelOverride(t *testing.T) {
	t.Run("returns empty when model flag not changed", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("model", "opus", "test model flag")

		got := resolveExploreModelOverride(cmd)
		if got != "" {
			t.Fatalf("resolveExploreModelOverride() = %q, want empty", got)
		}
	})

	t.Run("returns explicit model when model flag changed", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().String("model", "opus", "test model flag")
		if err := cmd.Flags().Set("model", "gemini-2.5-pro"); err != nil {
			t.Fatalf("setting model flag: %v", err)
		}

		got := resolveExploreModelOverride(cmd)
		if got != "gemini-2.5-pro" {
			t.Fatalf("resolveExploreModelOverride() = %q, want %q", got, "gemini-2.5-pro")
		}
	})

	t.Run("returns empty when model flag is missing", func(t *testing.T) {
		cmd := &cobra.Command{}

		got := resolveExploreModelOverride(cmd)
		if got != "" {
			t.Fatalf("resolveExploreModelOverride() = %q, want empty", got)
		}
	})
}
