package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/queue"
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
var queueCompletionOrder bool

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Show processing queue with model assignments",
	Long: `Display the beads Gromit will process, in order, with their assigned models based on priority and labels.

Also shows any blocked beads and the reason they can't be processed yet.`,
	RunE: showQueue,
}

func init() {
	queueCmd.Flags().BoolVar(&queueBySpec, "by-spec", false, "Group queue output by spec label")
	queueCmd.Flags().BoolVar(&queueCompletionOrder, "completion-order", false, "Show projected spec and bead completion order")
	rootCmd.AddCommand(queueCmd)
}

func showQueue(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	gromitDir := resolveGromitDir(cfg)
	executor, err := createQueuePipelineFn(cfg, gromitDir)
	if err != nil {
		return fmt.Errorf("creating pipeline: %w", err)
	}

	input := pipeline.QueueInput{
		LogsDir:        cfg.Paths.Logs,
		StuckThreshold: cfg.Loop.StuckBeadThreshold,
		GromitDir:      gromitDir,
	}

	result, err := executor.Queue(cmd.Context(), input)
	if err != nil {
		return err
	}
	if result == nil {
		result = &pipeline.QueueResult{}
	}

	if queueCompletionOrder {
		printCompletionOrder(cfg, result.Ready, result.All)
		return nil
	}

	printQueueByStatus(cfg, result.Ready, result.Blocked, result.Stuck, result.All, queueBySpec, isColorEnabled())
	return nil
}

var createQueuePipelineFn = createQueuePipeline

func createQueuePipeline(cfg *config.Config, gromitDir string) (queueExecutor, error) {
	p, err := newPipeline(cfg, gromitDir)
	if err != nil {
		return nil, err
	}
	return p, nil
}

type queueExecutor interface {
	Queue(context.Context, pipeline.QueueInput) (*pipeline.QueueResult, error)
}

func printQueueByStatus(cfg queueModelSelector, readyBeads, blockedBeads, stuckBeads, allBeads []*bead.Bead, bySpec bool, useColor bool) {
	if bySpec {
		printQueueBySpec(cfg, readyBeads, blockedBeads, stuckBeads, allBeads, useColor)
		return
	}

	if len(readyBeads) > 0 {
		fmt.Println("Queue (" + fmt.Sprintf("%d", len(readyBeads)) + " beads ready):")
		printBeadsBySpec(readyBeads, false, func(queueIndex int, b *bead.Bead) {
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
		printBeadsBySpec(blockedBeads, false, func(_ int, b *bead.Bead) {
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
		printBeadsBySpec(stuckBeads, false, func(_ int, b *bead.Bead) {
			line := fmt.Sprintf("  [P%d] %s  %s  (exceeded failure threshold)",
				b.Priority,
				b.ID,
				truncateTitle(b.Title, 30))
			fmt.Println(colorizeLine(line, ansiRed, useColor))
		})
	}
}

func printQueueBySpec(cfg queueModelSelector, readyBeads, blockedBeads, stuckBeads, allBeads []*bead.Bead, useColor bool) {
	if len(readyBeads) > 0 {
		fmt.Println("Queue (" + fmt.Sprintf("%d", len(readyBeads)) + " beads ready):")
	} else {
		fmt.Println("Queue: empty (no beads ready)")
	}

	readyBySpec := groupBeadsBySpec(readyBeads)
	blockedBySpec := groupBeadsBySpec(blockedBeads)
	stuckBySpec := groupBeadsBySpec(stuckBeads)
	specKeys := combinedSpecKeys(readyBySpec, blockedBySpec, stuckBySpec)

	fmt.Println("Queue by spec:")
	if len(specKeys) == 0 {
		fmt.Println("  (no spec groups to display)")
		return
	}
	readyIndex := 0
	for i, spec := range specKeys {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("  Spec: %s\n", formatSpecGroupName(spec))

		if specReady := readyBySpec[spec]; len(specReady) > 0 {
			fmt.Printf("    Ready (%d):\n", len(specReady))
			for _, b := range specReady {
				model := cfg.SelectModel(b.Priority, b.Labels)
				line := fmt.Sprintf("      %d. [P%d] %s  %s  → %s",
					readyIndex+1,
					b.Priority,
					b.ID,
					truncateTitle(b.Title, 30),
					model)
				fmt.Println(colorizeLine(line, ansiBold+ansiGreen, useColor))
				readyIndex++
			}
		}

		if specBlocked := blockedBySpec[spec]; len(specBlocked) > 0 {
			fmt.Printf("    Blocked (%d):\n", len(specBlocked))
			for _, b := range specBlocked {
				reasonStr := getReason(b, allBeads)
				line := fmt.Sprintf("      [P%d] %s  %s  (%s)",
					b.Priority,
					b.ID,
					truncateTitle(b.Title, 30),
					reasonStr)
				fmt.Println(colorizeLine(line, ansiWhite, useColor))
			}
		}

		if specStuck := stuckBySpec[spec]; len(specStuck) > 0 {
			fmt.Printf("    Stuck (%d):\n", len(specStuck))
			for _, b := range specStuck {
				line := fmt.Sprintf("      [P%d] %s  %s  (exceeded failure threshold)",
					b.Priority,
					b.ID,
					truncateTitle(b.Title, 30))
				fmt.Println(colorizeLine(line, ansiRed, useColor))
			}
		}
	}
}

type queueModelSelector interface {
	SelectModel(priority int, labels []string) string
}

type projectedSpecCompletion struct {
	Name            string
	CompletionIndex int
	Beads           []*bead.Bead
}

type completionKey struct {
	statusRank int
	unresolved int
	priority   int
	id         string
}

func printCompletionOrder(cfg queueModelSelector, readyBeads, allBeads []*bead.Bead) {
	order := projectBeadCompletionOrder(readyBeads, allBeads)
	specs := projectSpecCompletionOrder(order, allBeads)

	fmt.Println("Projected completion order (assuming no failures):")
	fmt.Println()
	fmt.Println("Specs:")
	for i, spec := range specs {
		fmt.Printf("  %d. %s (completes at bead #%d)\n", i+1, formatSpecGroupName(spec.Name), spec.CompletionIndex)
	}
	fmt.Println()
	fmt.Println("Beads:")
	for i, b := range order {
		model := cfg.SelectModel(b.Priority, b.Labels)
		spec := formatSpecGroupName(bead.FindSpecLabel(b.Labels))
		fmt.Printf("  %d. [%s] [P%d] %s  %s  → %s\n", i+1, spec, b.Priority, b.ID, truncateTitle(b.Title, 30), model)
	}
}

func projectSpecCompletionOrder(order, allBeads []*bead.Bead) []projectedSpecCompletion {
	remainingBySpec := make(map[string]int)
	specBeads := make(map[string][]*bead.Bead)
	for _, b := range allBeads {
		if b == nil {
			continue
		}
		spec := bead.FindSpecLabel(b.Labels)
		remainingBySpec[spec]++
	}

	completions := make([]projectedSpecCompletion, 0, len(remainingBySpec))
	completed := make(map[string]bool, len(remainingBySpec))
	for i, b := range order {
		if b == nil {
			continue
		}
		spec := bead.FindSpecLabel(b.Labels)
		specBeads[spec] = append(specBeads[spec], b)
		remainingBySpec[spec]--
		if remainingBySpec[spec] == 0 && !completed[spec] {
			completed[spec] = true
			completions = append(completions, projectedSpecCompletion{
				Name:            spec,
				CompletionIndex: i + 1,
				Beads:           specBeads[spec],
			})
		}
	}

	sort.Slice(completions, func(i, j int) bool {
		if completions[i].CompletionIndex != completions[j].CompletionIndex {
			return completions[i].CompletionIndex < completions[j].CompletionIndex
		}
		return completions[i].Name < completions[j].Name
	})
	return completions
}

func projectBeadCompletionOrder(readyBeads, allBeads []*bead.Bead) []*bead.Bead {
	readyNow := make(map[string]bool, len(readyBeads))
	for _, b := range readyBeads {
		if b != nil && strings.TrimSpace(b.ID) != "" {
			readyNow[b.ID] = true
		}
	}

	remaining := make(map[string]*bead.Bead, len(allBeads))
	for _, b := range allBeads {
		if b == nil || strings.TrimSpace(b.ID) == "" {
			continue
		}
		remaining[b.ID] = b
	}

	order := make([]*bead.Bead, 0, len(remaining))
	for len(remaining) > 0 {
		candidates := make([]*bead.Bead, 0, len(remaining))
		for _, b := range remaining {
			if unresolvedDepsCount(b, remaining) == 0 {
				candidates = append(candidates, b)
			}
		}
		if len(candidates) == 0 {
			for _, b := range remaining {
				candidates = append(candidates, b)
			}
		}

		sort.Slice(candidates, func(i, j int) bool {
			left := completionSortKey(candidates[i], remaining, readyNow)
			right := completionSortKey(candidates[j], remaining, readyNow)
			if left.statusRank != right.statusRank {
				return left.statusRank < right.statusRank
			}
			if left.unresolved != right.unresolved {
				return left.unresolved < right.unresolved
			}
			if left.priority != right.priority {
				return left.priority < right.priority
			}
			return left.id < right.id
		})

		next := candidates[0]
		order = append(order, next)
		delete(remaining, next.ID)
	}

	return order
}

func completionSortKey(b *bead.Bead, remaining map[string]*bead.Bead, readyNow map[string]bool) completionKey {
	statusRank := 2
	if strings.EqualFold(b.Status, "in_progress") {
		statusRank = 0
	} else if readyNow[b.ID] {
		statusRank = 1
	}
	return completionKey{
		statusRank: statusRank,
		unresolved: unresolvedDepsCount(b, remaining),
		priority:   b.Priority,
		id:         b.ID,
	}
}

func unresolvedDepsCount(b *bead.Bead, remaining map[string]*bead.Bead) int {
	if b == nil {
		return 0
	}
	seen := make(map[string]bool)
	addDep := func(depID string) {
		depID = strings.TrimSpace(depID)
		if depID == "" || seen[depID] {
			return
		}
		seen[depID] = true
	}

	if b.Parent != "" {
		addDep(b.Parent)
	}
	for _, depID := range queue.DependencyIDs(b.BlockedBy) {
		addDep(depID)
	}
	for _, depID := range queue.DependencyIDs(b.DependsOn) {
		addDep(depID)
	}
	for _, depID := range queue.DependencyIDs(b.Dependencies) {
		addDep(depID)
	}

	unresolved := 0
	for depID := range seen {
		if _, ok := remaining[depID]; ok {
			unresolved++
		}
	}
	return unresolved
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

func combinedSpecKeys(groups ...map[string][]*bead.Bead) []string {
	combined := make(map[string][]*bead.Bead)
	for _, group := range groups {
		for key := range group {
			combined[key] = nil
		}
	}
	return orderedSpecKeys(combined)
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

// getReason returns a human-readable reason why a bead is blocked
func getReason(b *bead.Bead, allBeads []*bead.Bead) string {
	return queue.GetReason(b, allBeads)
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
