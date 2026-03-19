package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/workspace"
	"github.com/spf13/cobra"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Spec management commands",
}

// ProjectConfig holds project configuration loaded from project.json.
type ProjectConfig struct {
	RepoPath string `json:"repo_path"`
	SpecsDir string `json:"specs_dir"`
}

// LoadProjectConfig loads a ProjectConfig from project.json in the given directory.
func LoadProjectConfig(dir string) (*ProjectConfig, error) {
	path := filepath.Join(dir, "project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}
	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse project config: %w", err)
	}
	return &cfg, nil
}

// ResolveProjectConfigPath resolves the project cell directory from the workspace root and project name.
func ResolveProjectConfigPath(root workspace.Root, projectName string) (string, error) {
	dir := root.ProjectCell(projectName)
	if _, err := os.Stat(filepath.Join(dir, "project.json")); err != nil {
		return "", fmt.Errorf("project %q not found at %s: %w", projectName, dir, err)
	}
	return dir, nil
}

// DiscoverSpecs scans a directory for .md spec files and returns their base names (without extension).
func DiscoverSpecs(specsDir string) ([]string, error) {
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return nil, fmt.Errorf("read specs dir: %w", err)
	}
	var specs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) == ".md" {
			name := strings.TrimSuffix(e.Name(), ".md")
			specs = append(specs, name)
		}
	}
	if specs == nil {
		specs = []string{}
	}
	return specs, nil
}

// ParseDoneDate extracts a completion date from a "DONE YYYY-MM-DD" prefix.
// Returns (time, true) if the content starts with "DONE" (even if date is malformed or missing),
// or (zero, false) if there is no DONE prefix.
func ParseDoneDate(content string) (time.Time, bool) {
	if !strings.HasPrefix(content, "DONE ") && !strings.HasPrefix(content, "DONE\n") && content != "DONE" {
		return time.Time{}, false
	}

	// Extract the part after "DONE"
	afterDone := strings.TrimPrefix(content, "DONE")
	afterDone = strings.TrimSpace(afterDone)

	// If nothing after DONE or just whitespace, return zero time with true
	if afterDone == "" {
		return time.Time{}, true
	}

	// Try to extract the date (first token)
	dateStr := strings.Fields(afterDone)[0]

	// Try to parse the date
	parsedTime, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		// Malformed date, but DONE prefix exists so return true with zero time
		return time.Time{}, true
	}

	return parsedTime, true
}

// DeriveSpecStatus derives the aggregate status of a spec from its run history.
// specID is included for future per-spec filtering; currently unused.
func DeriveSpecStatus(specID string, runs []runstore.RunState) string {
	if len(runs) == 0 {
		return "ready"
	}
	// "completed" takes highest priority — human review accepted the work.
	// NOTE: StatusCompleted will be actively set when Spec 0002b adds acceptance gates.
	for _, r := range runs {
		if r.Status == runstore.StatusCompleted {
			return "completed"
		}
	}
	for _, r := range runs {
		if r.Status == runstore.StatusReadyForReview {
			return "ready_for_review"
		}
	}
	for _, r := range runs {
		if r.Status == runstore.StatusRunning {
			return "running"
		}
	}
	for _, r := range runs {
		if r.Status == runstore.StatusNeedsHuman || r.Status == runstore.StatusBlocked {
			return "needs_attention"
		}
	}
	return "ready"
}

// DeriveSpecStatusFromContent derives spec status considering both runs and content.
// If content starts with "DONE", returns "done" regardless of run status.
// Otherwise, if content starts with "DRAFT", returns "draft" regardless of run status.
// specID is included for future per-spec filtering; currently unused.
func DeriveSpecStatusFromContent(specID string, runs []runstore.RunState, content string) string {
	if _, ok := ParseDoneDate(content); ok {
		return "done"
	}
	if strings.HasPrefix(content, "DRAFT") {
		return "draft"
	}
	return DeriveSpecStatus(specID, runs)
}

// sortSpecsByDone sorts specs so that non-done specs come first (in original order),
// followed by done specs (in original order).
func sortSpecsByDone(specs []string, contents map[string]string) []string {
	var nonDone, done []string
	for _, spec := range specs {
		content := contents[spec]
		if _, ok := ParseDoneDate(content); ok {
			done = append(done, spec)
		} else {
			nonDone = append(nonDone, spec)
		}
	}
	return append(nonDone, done...)
}

// formatSpecStatusWithDate formats the spec status, including completion date for done specs.
// If the spec is done with a valid date, returns "done (YYYY-MM-DD)".
// If the spec is done without a valid date, returns "done".
// Otherwise, derives status normally.
func formatSpecStatusWithDate(specID string, runs []runstore.RunState, content string) string {
	parsedTime, ok := ParseDoneDate(content)
	if ok {
		if parsedTime.IsZero() {
			return "done"
		}
		return fmt.Sprintf("done (%s)", parsedTime.Format("2006-01-02"))
	}
	if strings.HasPrefix(content, "DRAFT") {
		return "draft"
	}
	return DeriveSpecStatus(specID, runs)
}

// newSpecListCmd creates the `spec list` command.
func newSpecListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List specs and their statuses",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			project, _ := cmd.Flags().GetString("project")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			specsDir, _ := cmd.Flags().GetString("specs-dir")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)

			if specsDir == "" {
				resolver := workspace.NewEnvResolver()
				root, err := resolver.Resolve()
				if err != nil {
					return fmt.Errorf("resolve workspace root: %w", err)
				}
				projectDir, err := ResolveProjectConfigPath(root, project)
				if err != nil {
					return fmt.Errorf("resolve project config: %w", err)
				}
				cfg, err := LoadProjectConfig(projectDir)
				if err != nil {
					return fmt.Errorf("load project config: %w", err)
				}
				specsDir = cfg.SpecsDir
				if specsDir == "" && cfg.RepoPath != "" {
					specsDir = filepath.Join(cfg.RepoPath, "docs", "specs")
				}
			}

			specs, err := DiscoverSpecs(specsDir)
			if err != nil {
				return err
			}

			runs, err := store.List(project)
			if err != nil {
				return err
			}

			// Convert []*RunState to []RunState for DeriveSpecStatus.
			runValues := make([]runstore.RunState, len(runs))
			for i, r := range runs {
				runValues[i] = *r
			}

			var b strings.Builder
			tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
			// Load all spec contents for sorting.
			specContents := make(map[string]string)
			for _, spec := range specs {
				content, _ := os.ReadFile(filepath.Join(specsDir, spec+".md"))
				specContents[spec] = string(content)
			}

			// Sort specs with done ones at the bottom.
			sortedSpecs := sortSpecsByDone(specs, specContents)

			fmt.Fprintln(tw, "SPEC\tSTATUS\tLAST RUN")
			for _, spec := range sortedSpecs {
				// Filter runs for this spec.
				var specRuns []runstore.RunState
				for _, r := range runValues {
					if r.SpecID == spec {
						specRuns = append(specRuns, r)
					}
				}
				content := specContents[spec]
				status := formatSpecStatusWithDate(spec, specRuns, content)
				lastRun := "-"
				if len(specRuns) > 0 {
					// Find the most recent run.
					latest := specRuns[0]
					for _, r := range specRuns[1:] {
						if r.StartedAt.After(latest.StartedAt) {
							latest = r
						}
					}
					lastRun = latest.RunID + " " + latest.StartedAt.Format("2006-01-02 15:04:05")
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n", spec, status, lastRun)
			}
			tw.Flush()
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	cmd.Flags().String("project", "", "Project name")
	cmd.Flags().String("store-dir", "", "Override store directory (for testing)")
	cmd.Flags().String("specs-dir", "", "Override specs directory (for testing)")
	_ = cmd.MarkFlagRequired("project")
	return cmd
}
