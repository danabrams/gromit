package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitadapter "github.com/danabrams/gromit/internal/v2/adapter/git"
	"github.com/danabrams/gromit/internal/v2/pipeline"
	"github.com/spf13/cobra"
)

// debug2AgentLaunchFn is injectable for tests. It launches the agent with a prompt
// file in the given directory.
// t.Cleanup must restore the original value in tests that override this.
var debug2AgentLaunchFn = func(promptPath, dir string) error {
	cfg, _ := loadConfig()
	selectedAgent, err := resolveCommandAgent(cfg, "debug", "", false)
	if err != nil {
		return fmt.Errorf("resolving agent: %w", err)
	}
	return selectedAgent.LaunchInDir(promptPath, dir)
}

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

// buildDebug2Prompt assembles a diagnostic prompt from the spec name, worktree path,
// events, and commit history. commits is a slice of [hash, message] pairs.
func buildDebug2Prompt(specName, wtPath string, events []map[string]interface{}, commits [][2]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Debug Session: %s\n\n", specName))
	sb.WriteString(fmt.Sprintf("Worktree: %s\n\n", wtPath))

	sb.WriteString("### Commit History\n\n")
	for _, c := range commits {
		sb.WriteString(fmt.Sprintf("  %s %s\n", c[0], c[1]))
	}
	sb.WriteString("\n")

	sb.WriteString("### Event Log\n\n")
	for _, e := range events {
		data, _ := json.Marshal(e)
		sb.WriteString(fmt.Sprintf("  %s\n", string(data)))
	}
	sb.WriteString("\n")

	failEvent := findFailureEvent(events)
	if failEvent != nil {
		sb.WriteString("### Failure Point\n\n")
		data, _ := json.Marshal(failEvent)
		sb.WriteString(fmt.Sprintf("  %s\n\n", string(data)))
	}

	sb.WriteString("## Task\n\nDiagnose the failure above and produce a fix.\n")
	return sb.String()
}

// findFailureEvent returns the first event with decision "Fail", or nil if none found.
func findFailureEvent(events []map[string]interface{}) map[string]interface{} {
	for _, e := range events {
		if e["decision"] == "Fail" {
			return e
		}
	}
	return nil
}

// debug2Impl contains the testable core of the debug2 command.
func debug2Impl(specName, gromitDir string) error {
	wtPath, err := resolveDebug2Worktree(gromitDir, specName)
	if err != nil {
		return err
	}

	events, err := readDebug2EventLog(wtPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading event log: %w", err)
	}

	gitAdapter := gitadapter.NewExecGitAdapter(".", gromitDir)
	logEntries, err := gitAdapter.Log(context.Background(), wtPath, 100)
	if err != nil {
		logEntries = nil // non-fatal: proceed without commit history
	}

	commits := make([][2]string, 0, len(logEntries))
	for _, e := range logEntries {
		hash := e.Hash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		msg := e.Message
		if _, ok := pipeline.ParseCommitMessage(e.Message); ok {
			msg = e.Message
		}
		commits = append(commits, [2]string{hash, msg})
	}

	prompt := buildDebug2Prompt(specName, wtPath, events, commits)

	tmpFile, err := os.CreateTemp("", "debug2-prompt-*.md")
	if err != nil {
		return fmt.Errorf("creating temp prompt file: %w", err)
	}
	promptPath := tmpFile.Name()
	defer os.Remove(promptPath)

	if _, err := tmpFile.WriteString(prompt); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing prompt file: %w", err)
	}
	tmpFile.Close()

	return debug2AgentLaunchFn(promptPath, wtPath)
}

func debug2RunE(cmd *cobra.Command, args []string) error {
	specName := args[0]
	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("loading config: %w", err)
	}
	gromitDir := resolveGromitDir(cfg)
	return debug2Impl(specName, gromitDir)
}
