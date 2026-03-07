package main

import (
	"bufio"
	"encoding/json"
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

// readDebug2EventLog reads and parses the JSONL event log from a spec worktree.
// Each line is decoded as a map[string]interface{}.
func readDebug2EventLog(wtPath string) ([]map[string]interface{}, error) {
	eventsPath := filepath.Join(wtPath, ".gromit", "v2", "events.jsonl")
	f, err := os.Open(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("opening event log: %w", err)
	}
	defer f.Close()

	var events []map[string]interface{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parsing event line: %w", err)
		}
		events = append(events, entry)
	}
	return events, scanner.Err()
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
