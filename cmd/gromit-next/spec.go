package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/spf13/cobra"
)

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Spec management commands",
}

// ProjectConfig holds project configuration loaded from project.json.
type ProjectConfig struct {
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

// DeriveSpecStatus derives the aggregate status of a spec from its run history.
// specID is included for future per-spec filtering; currently unused.
func DeriveSpecStatus(specID string, runs []runstore.RunState) string {
	if len(runs) == 0 {
		return "ready"
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
// If content starts with "DRAFT", returns "draft" regardless of run status.
// specID is included for future per-spec filtering; currently unused.
func DeriveSpecStatusFromContent(specID string, runs []runstore.RunState, content string) string {
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
			project, _ := cmd.Flags().GetString("project")
			storeDir, _ := cmd.Flags().GetString("store-dir")
			specsDir, _ := cmd.Flags().GetString("specs-dir")

			if storeDir == "" {
				storeDir = ".gromit-next"
			}

			store := runstore.NewStore(storeDir)

			if specsDir == "" {
				cfg, err := LoadProjectConfig(".")
				if err != nil {
					return fmt.Errorf("load project config: %w", err)
				}
				specsDir = cfg.SpecsDir
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
			fmt.Fprintln(tw, "SPEC\tSTATUS\tLAST RUN")
			for _, spec := range specs {
				// Filter runs for this spec.
				var specRuns []runstore.RunState
				for _, r := range runValues {
					if r.SpecID == spec {
						specRuns = append(specRuns, r)
					}
				}
				content, _ := os.ReadFile(filepath.Join(specsDir, spec+".md"))
				status := DeriveSpecStatusFromContent(spec, specRuns, string(content))
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
