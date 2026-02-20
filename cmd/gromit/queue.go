package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/logger"
	"github.com/spf13/cobra"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiGreen = "\x1b[32m"
	ansiWhite = "\x1b[37m"
	ansiRed   = "\x1b[31m"
)

var queueBySpec bool

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Show processing queue with model assignments",
	Long: `Display the beads Gromit will process, in order, with their assigned models based on priority and labels.

Also shows any blocked beads and the reason they can't be processed yet.`,
	RunE: showQueue,
}

func init() {
	queueCmd.Flags().BoolVar(&queueBySpec, "by-spec", false, "Group queue output by spec label")
	rootCmd.AddCommand(queueCmd)
}

func showQueue(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	bc, err := bead.NewClient()
	if err != nil {
		return fmt.Errorf("creating bead client: %w", err)
	}

	// Get ready beads (unblocked, type=task to exclude epics)
	readyBeads, err := getReadyBeads(bc)
	if err != nil {
		return fmt.Errorf("getting ready beads: %w", err)
	}

	// Get all open beads to identify blocked ones
	allBeads, err := bc.List()
	if err != nil {
		return fmt.Errorf("getting all beads: %w", err)
	}

	var beadStats map[string]logger.BeadStats
	beadStats, err = logger.ReadPerBeadStats(cfg.Paths.Logs)
	if err != nil {
		beadStats = map[string]logger.BeadStats{}
	}

	readyBeads, blockedBeads, stuckBeads := partitionQueueBeads(
		readyBeads,
		allBeads,
		beadStats,
		cfg.Loop.StuckBeadThreshold,
	)
	blockedBeads = enrichBlockedBeads(bc, blockedBeads)

	printQueueByStatus(cfg, readyBeads, blockedBeads, stuckBeads, allBeads, queueBySpec, isColorEnabled())
	return nil
}

func printQueueByStatus(cfg queueModelSelector, readyBeads, blockedBeads, stuckBeads, allBeads []*bead.Bead, bySpec bool, useColor bool) {
	if len(readyBeads) > 0 {
		fmt.Println("Queue (" + fmt.Sprintf("%d", len(readyBeads)) + " beads ready):")
		printBeadsBySpec(readyBeads, bySpec, func(queueIndex int, b *bead.Bead) {
			model := cfg.SelectModel(b.Priority, b.Labels)
			line := fmt.Sprintf("  %d. [P%d] %s  %s  → %s",
				queueIndex+1,
				b.Priority,
				b.ID,
				truncateTitle(b.Title, 30),
				model)
			fmt.Println(colorizeLine(line, ansiBold+ansiGreen, useColor))
		})
	} else {
		fmt.Println("Queue: empty (no beads ready)")
	}

	if len(blockedBeads) > 0 {
		fmt.Println()
		fmt.Println("Blocked (" + fmt.Sprintf("%d", len(blockedBeads)) + "):")
		printBeadsBySpec(blockedBeads, bySpec, func(_ int, b *bead.Bead) {
			reasonStr := getReason(b, allBeads)
			line := fmt.Sprintf("  [P%d] %s  %s  (%s)",
				b.Priority,
				b.ID,
				truncateTitle(b.Title, 30),
				reasonStr)
			fmt.Println(colorizeLine(line, ansiWhite, useColor))
		})
	}

	if len(stuckBeads) > 0 {
		fmt.Println()
		fmt.Println("Stuck (" + fmt.Sprintf("%d", len(stuckBeads)) + "):")
		printBeadsBySpec(stuckBeads, bySpec, func(_ int, b *bead.Bead) {
			line := fmt.Sprintf("  [P%d] %s  %s  (exceeded failure threshold)",
				b.Priority,
				b.ID,
				truncateTitle(b.Title, 30))
			fmt.Println(colorizeLine(line, ansiRed, useColor))
		})
	}
}

type queueModelSelector interface {
	SelectModel(priority int, labels []string) string
}

func printBeadsBySpec(beads []*bead.Bead, bySpec bool, printFn func(queueIndex int, b *bead.Bead)) {
	if !bySpec {
		for i, b := range beads {
			printFn(i, b)
		}
		return
	}

	grouped := groupBeadsBySpec(beads)
	keys := orderedSpecKeys(grouped)
	queueIndex := 0
	for _, key := range keys {
		fmt.Printf("  Spec: %s\n", formatSpecGroupName(key))
		for _, b := range grouped[key] {
			printFn(queueIndex, b)
			queueIndex++
		}
	}
}

func groupBeadsBySpec(beads []*bead.Bead) map[string][]*bead.Bead {
	grouped := make(map[string][]*bead.Bead)
	for _, b := range beads {
		spec := bead.FindSpecLabel(b.Labels)
		grouped[spec] = append(grouped[spec], b)
	}
	return grouped
}

func orderedSpecKeys(grouped map[string][]*bead.Bead) []string {
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "" {
			return false
		}
		if keys[j] == "" {
			return true
		}
		return keys[i] < keys[j]
	})
	return keys
}

func formatSpecGroupName(spec string) string {
	if strings.TrimSpace(spec) == "" {
		return "(none)"
	}
	return spec
}

func isColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func colorizeLine(line, color string, useColor bool) string {
	if !useColor {
		return line
	}
	return color + line + ansiReset
}

// getReadyBeads fetches ready beads using bd ready --json
// This simulates getting multiple ready beads instead of just one
func getReadyBeads(bc *bead.Client) ([]*bead.Bead, error) {
	if bc == nil {
		return nil, fmt.Errorf("bead client is nil")
	}
	readyBeads, err := bc.ListReadyWork()
	if err != nil {
		return nil, fmt.Errorf("getting ready beads: %w", err)
	}
	return readyBeads, nil
}

func partitionQueueBeads(
	readyBeads, allBeads []*bead.Bead,
	beadStats map[string]logger.BeadStats,
	stuckThreshold int,
) (ready []*bead.Bead, blocked []*bead.Bead, stuck []*bead.Bead) {
	stuckMap := findStuckBeadIDs(beadStats, stuckThreshold)

	readyMap := make(map[string]bool, len(readyBeads))
	for _, b := range readyBeads {
		readyMap[b.ID] = true
		if !stuckMap[b.ID] {
			ready = append(ready, b)
		}
	}

	for _, b := range allBeads {
		if stuckMap[b.ID] {
			stuck = append(stuck, b)
			continue
		}
		if !readyMap[b.ID] {
			blocked = append(blocked, b)
		}
	}

	return ready, blocked, stuck
}

func findStuckBeadIDs(beadStats map[string]logger.BeadStats, threshold int) map[string]bool {
	stuck := make(map[string]bool)
	if threshold <= 0 {
		return stuck
	}
	for beadID, stats := range beadStats {
		if stats.Failures >= threshold {
			stuck[beadID] = true
		}
	}
	return stuck
}

// findBlockedBeads returns beads that are open but not in the ready list.
func findBlockedBeads(readyBeads, allBeads []*bead.Bead) []*bead.Bead {
	readyMap := make(map[string]bool)
	for _, b := range readyBeads {
		readyMap[b.ID] = true
	}

	blocked := []*bead.Bead{}
	for _, b := range allBeads {
		if !readyMap[b.ID] {
			blocked = append(blocked, b)
		}
	}
	return blocked
}

// getReason returns a human-readable reason why a bead is blocked
func getReason(b *bead.Bead, allBeads []*bead.Bead) string {
	if b == nil {
		return "unknown"
	}

	if b.Parent != "" {
		// Check if parent still exists in open beads
		for _, openB := range allBeads {
			if openB.ID == b.Parent {
				return fmt.Sprintf("blocked by: %s", b.Parent)
			}
		}
		return fmt.Sprintf("blocked by parent: %s", b.Parent)
	}

	if depIDs := dependencyIDs(b.BlockedBy); len(depIDs) > 0 {
		return fmt.Sprintf("blocked by: %s", strings.Join(depIDs, ", "))
	}
	if depIDs := dependencyIDs(b.DependsOn); len(depIDs) > 0 {
		return fmt.Sprintf("blocked by: %s", strings.Join(depIDs, ", "))
	}
	if depIDs := dependencyIDs(b.Dependencies); len(depIDs) > 0 {
		return fmt.Sprintf("blocked by: %s", strings.Join(depIDs, ", "))
	}
	if b.DependencyCount != nil && *b.DependencyCount > 0 {
		return fmt.Sprintf("blocked by %d dependencies", *b.DependencyCount)
	}

	return "dependencies unresolved"
}

func dependencyIDs(deps []bead.Dependency) []string {
	ids := make([]string, 0, len(deps))
	for _, dep := range deps {
		if strings.TrimSpace(dep.ID) == "" {
			continue
		}
		ids = append(ids, dep.ID)
	}
	return ids
}

func enrichBlockedBeads(bc *bead.Client, blocked []*bead.Bead) []*bead.Bead {
	if bc == nil || len(blocked) == 0 {
		return blocked
	}
	enriched := make([]*bead.Bead, 0, len(blocked))
	for _, b := range blocked {
		if b == nil {
			continue
		}
		needsDetails := b.Parent == "" && len(b.Dependencies) == 0 && len(b.BlockedBy) == 0 && len(b.DependsOn) == 0
		if !needsDetails {
			enriched = append(enriched, b)
			continue
		}
		full, err := bc.Show(b.ID)
		if err == nil && full != nil {
			enriched = append(enriched, full)
			continue
		}
		enriched = append(enriched, b)
	}
	return enriched
}

// truncateTitle returns a title truncated to maxLen characters with ellipsis if needed
func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	if maxLen <= 3 {
		return title[:maxLen]
	}
	return title[:maxLen-3] + "..."
}
