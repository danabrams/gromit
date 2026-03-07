package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var debug2Cmd = &cobra.Command{
	Use:   "debug2 <spec-name>",
	Short: "Diagnose and fix a failed v2 spec execution",
	Args:  cobra.ExactArgs(1),
	RunE:  debug2RunE,
}

func init() {
	rootCmd.AddCommand(debug2Cmd)
}

// resolveDebug2Worktree returns the path to the preserved spec worktree, or
// an error if no such worktree exists.
func resolveDebug2Worktree(gromitDir, specName string) (string, error) {
	wtPath := filepath.Join(gromitDir, "spec-worktrees", specName)
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return "", fmt.Errorf("no preserved worktree found for spec %q at %s", specName, wtPath)
	}
	return wtPath, nil
}

func debug2RunE(cmd *cobra.Command, args []string) error {
	specName := args[0]
	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}
	gromitDir := resolveGromitDir(cfg)

	wtPath, err := resolveDebug2Worktree(gromitDir, specName)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Spec: %s\nWorktree: %s\n", specName, wtPath)
	return nil
}
