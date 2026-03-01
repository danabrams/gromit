package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

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
	if len(args) == 1 {
		return fmt.Errorf("spec detail view not implemented yet")
	}

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	store, err := newPRStateStoreFn(gromitDir)
	if err != nil {
		return fmt.Errorf("creating spec PR state store: %w", err)
	}

	client := newPRClientFn()
	if client == nil {
		return fmt.Errorf("pr client is not configured")
	}

	return runPRSList(cmd.Context(), cmd.OutOrStdout(), store, client)
}

func runPRSList(ctx context.Context, w io.Writer, store specmerge.PRStateStore, client specmerge.PRClient) error {
	states, err := store.List(ctx)
	if err != nil {
		return fmt.Errorf("listing spec PR states: %w", err)
	}

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

type prListRow struct {
	spec   string
	number int
	title  string
}

func isOpenPR(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "open")
}
