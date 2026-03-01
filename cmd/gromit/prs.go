package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/runner/specmerge"
	"github.com/spf13/cobra"
)

var (
	prsCmd = &cobra.Command{
		Use:   "prs [spec-name]",
		Short: "Manage spec PRs",
		Long:  "List open spec PRs or inspect a spec's pull request details.",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runPRS,
	}

	newPRStateStoreFn = specmerge.NewPRStateStoreFile
	newPRClientFn     = func() specmerge.PRClient { return specmerge.NewGhCLIClient(nil) }
)

func init() {
	rootCmd.AddCommand(prsCmd)
}

func runPRS(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	store, err := newPRStateStoreFn(gromitDir)
	if err != nil {
		return fmt.Errorf("creating spec PR state store: %w", err)
	}

	states, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing spec PR states: %w", err)
	}

	client := newPRClientFn()
	if client == nil {
		return fmt.Errorf("pr client is not configured")
	}

	if len(args) == 0 {
		return renderPRSList(ctx, cmd.OutOrStdout(), states, client)
	}

	return renderPRSDetail(ctx, cmd.OutOrStdout(), states, client, args[0])
}

func renderPRSList(ctx context.Context, w io.Writer, states []*specmerge.PRState, client specmerge.PRClient) error {

	var rows []prListRow
	for _, state := range states {
		if state == nil {
			continue
		}
		if state.PRRef.Number == 0 {
			continue
		}

		status, err := client.GetPR(ctx, state.PRRef)
		if err != nil {
			return err
		}
		if !isOpenPR(status.State) {
			continue
		}

		specName := strings.TrimSpace(state.SpecName)
		if specName == "" {
			specName = "(unknown)"
		}

		rows = append(rows, prListRow{
			spec:   specName,
			number: status.Number,
			title:  status.Title,
		})
	}

	if len(rows) == 0 {
		fmt.Fprintln(w, "No open spec PRs found.")
		return nil
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].spec < rows[j].spec
	})

	fmt.Fprintf(w, "Open spec PRs (%d):\n", len(rows))
	for _, row := range rows {
		fmt.Fprintf(w, "  %s  #%d  %s\n", row.spec, row.number, row.title)
	}

	return nil
}

func renderPRSDetail(ctx context.Context, w io.Writer, states []*specmerge.PRState, client specmerge.PRClient, specName string) error {
	target := strings.TrimSpace(specName)
	if target == "" {
		return fmt.Errorf("spec name is required")
	}

	var found *specmerge.PRState
	for _, state := range states {
		if state == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(state.SpecName), target) {
			found = state
			break
		}
	}
	if found == nil {
		return fmt.Errorf("spec %q not found", specName)
	}
	if found.PRRef.Number == 0 {
		return fmt.Errorf("spec %q does not have an associated PR", specName)
	}

	status, err := client.GetPR(ctx, found.PRRef)
	if err != nil {
		return fmt.Errorf("get pr for spec %q: %w", specName, err)
	}

	checks, err := client.ListChecks(ctx, found.PRRef)
	if err != nil {
		return fmt.Errorf("list checks for spec %q: %w", specName, err)
	}

	specLabel := strings.TrimSpace(found.SpecName)
	if specLabel == "" {
		specLabel = target
	}

	fmt.Fprintf(w, "Spec: %s\n", specLabel)
	fmt.Fprintf(w, "PR: #%d\n", status.Number)
	fmt.Fprintf(w, "Title: %s\n", status.Title)
	fmt.Fprintf(w, "State: %s\n", strings.ToLower(strings.TrimSpace(status.State)))
	fmt.Fprintf(w, "Outcome: %s\n", found.Outcome)
	fmt.Fprintf(w, "Awaiting approval: %s\n", yesNo(found.AwaitingApproval))

	lastUpdated := "n/a"
	if !found.LastUpdated.IsZero() {
		lastUpdated = found.LastUpdated.UTC().Format(time.RFC3339)
	}
	fmt.Fprintf(w, "Last updated: %s\n", lastUpdated)

	printStageResults(w, found.StageResults)
	printChecks(w, checks)

	return nil
}

func printStageResults(w io.Writer, results []specmerge.StageResult) {
	fmt.Fprintf(w, "Stage results (%d):\n", len(results))
	if len(results) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	for _, stage := range results {
		summary := "<none>"
		if stage.ReviewResult != nil && strings.TrimSpace(stage.ReviewResult.Summary) != "" {
			summary = stage.ReviewResult.Summary
		}
		fmt.Fprintf(w, "  %s (tier=%s) - passed=%t - summary=%s\n", stage.StageName, stage.Tier, stage.Passed, summary)
	}
}

func printChecks(w io.Writer, checks []specmerge.CheckStatus) {
	fmt.Fprintf(w, "Checks (%d):\n", len(checks))
	if len(checks) == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	for _, check := range checks {
		fmt.Fprintf(w, "  %s - status=%s - conclusion=%s\n", check.Name, check.Status, check.Conclusion)
	}
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

type prListRow struct {
	spec   string
	number int
	title  string
}

func isOpenPR(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "open")
}
