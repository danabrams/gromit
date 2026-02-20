package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/spf13/cobra"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiGreen = "\x1b[32m"
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

	// Identify blocked beads (open but not ready)
	blockedBeads := findBlockedBeads(readyBeads, allBeads)

	printQueueByStatus(cfg, readyBeads, blockedBeads, allBeads, queueBySpec, isColorEnabled())
	return nil
}

func printQueueByStatus(cfg queueModelSelector, readyBeads, blockedBeads, allBeads []*bead.Bead, bySpec bool, useColor bool) {
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

	// Note: bd ready doesn't have a way to get all ready beads at once,
	// only the next one. We can use bd list with a filter to get all open
	// beads, then filter for ready ones by checking which don't have parent
	// relationships blocking them. For now, we'll just use the list and
	// identify the ones that would be ready.

	// Get all open beads sorted by priority
	allOpen, err := bc.List()
	if err != nil {
		return nil, fmt.Errorf("getting all open beads: %w", err)
	}

	readyBeads := []*bead.Bead{}
	parentMap := make(map[string]bool)

	// Build a map of beads that are parents (not yet closed)
	for _, b := range allOpen {
		if b.Parent != "" {
			parentMap[b.Parent] = true
		}
	}

	// Filter: a bead is ready if it has no parent, or its parent is closed
	for _, b := range allOpen {
		// If no parent, it's ready
		if b.Parent == "" {
			readyBeads = append(readyBeads, b)
			continue
		}

		// If parent exists in open beads, this is blocked
		isBlocked := false
		for _, openB := range allOpen {
			if openB.ID == b.Parent {
				isBlocked = true
				break
			}
		}

		if !isBlocked {
			// Parent is closed, so this is ready
			readyBeads = append(readyBeads, b)
		}
	}

	return readyBeads, nil
}

// findBlockedBeads returns beads that are open but not in the ready list
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
		// Parent doesn't exist in open beads, so it should be ready
		// (this shouldn't happen in normal flow)
		return "parent closed but still blocked"
	}

	return "unknown reason"
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
