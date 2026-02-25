package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/frontmatter"
)

// PipelineStatus represents the current state of the gromit pipeline
type PipelineStatus struct {
	UnrefinedCount     int      // Number of backlog items not yet refined
	UnrefinedIdeas     []string // Text of unrefined ideas
	UnplannedSpecs     []string // Names of specs without corresponding plans
	UndecomposedPlans  []string // Names of plans not yet decomposed
	ReadyBeadCount     int      // Number of ready beads
	ReadyBeads         []string // IDs of ready beads (up to 3 shown, rest summarized)
	InProgressCount    int      // Number of beads currently in progress
	BlockedCount       int      // Number of blocked beads (open but dependencies not met)
	DeferredCount      int      // Number of deferred beads
	ClosedCount        int      // Number of closed beads
	ClosedThisRunCount int      // Number of beads closed during this run
	HasRunInfo         bool     // Whether run start time is available (for "this run" counts)
	Recommendation     string   // Suggested next action
}

// ReadStatusWithDeps reads pipeline state using dependency-injected clients
func ReadStatusWithDeps(gromitDir, specsDir, plansDir string, startedAt *time.Time, backlogClient BacklogClient, beadQueryClient BeadQueryClient) (*PipelineStatus, error) {
	status := &PipelineStatus{
		UnrefinedIdeas:    []string{},
		UnplannedSpecs:    []string{},
		UndecomposedPlans: []string{},
		ReadyBeads:        []string{},
	}

	// Read backlog for unrefined ideas using injected client
	ideas, err := backlogClient.List()
	if err != nil {
		return nil, fmt.Errorf("reading backlog: %w", err)
	}

	for _, idea := range ideas {
		if idea.Status != "refined" {
			status.UnrefinedCount++
			status.UnrefinedIdeas = append(status.UnrefinedIdeas, idea.Text)
		}
	}

	// Scan specs directory for unplanned specs
	specFiles, err := findMarkdownFiles(specsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading specs directory: %w", err)
	}

	for _, specFile := range specFiles {
		specName := strings.TrimSuffix(filepath.Base(specFile), ".md")
		planFile := filepath.Join(plansDir, specName+".md")

		// Check if corresponding plan exists
		if _, err := os.Stat(planFile); os.IsNotExist(err) {
			status.UnplannedSpecs = append(status.UnplannedSpecs, specName)
		}
	}
	sort.Strings(status.UnplannedSpecs)

	// Scan plans directory for undecomposed plans
	planFiles, err := findMarkdownFiles(plansDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading plans directory: %w", err)
	}

	for _, planFile := range planFiles {
		planName := strings.TrimSuffix(filepath.Base(planFile), ".md")

		// Parse frontmatter to check decomposed field
		fm, _, err := frontmatter.ReadFile(planFile)
		if err != nil {
			return nil, fmt.Errorf("reading plan frontmatter %s: %w", planName, err)
		}

		// Check if decomposed field is missing or false
		decomposed, ok := fm["decomposed"].(bool)
		if !ok || !decomposed {
			status.UndecomposedPlans = append(status.UndecomposedPlans, planName)
		}
	}
	sort.Strings(status.UndecomposedPlans)

	if startedAt != nil {
		status.HasRunInfo = true
	}

	// Count ready/closed/in-progress beads only when a bd repo is present.
	// This avoids repeated expensive shellouts in non-bd directories.
	repoRoot := filepath.Dir(gromitDir)
	if hasBeadsRepo(repoRoot) {
		// Best-effort: if client creation or any command fails, counts remain at zero.
		if beadQueryClient != nil {
			status.ReadyBeads, status.ReadyBeadCount = listReadyBeads(beadQueryClient)

			// In-progress count
			if count, err := beadQueryClient.CountByStatus("in_progress"); err == nil {
				status.InProgressCount = count
			}

			// Deferred count
			if count, err := beadQueryClient.CountByStatus("deferred"); err == nil {
				status.DeferredCount = count
			}

			// Closed count
			if count, err := beadQueryClient.CountByStatus("closed"); err == nil {
				status.ClosedCount = count
			}

			// Blocked count = open - ready
			// Open beads include both ready and blocked (those with unmet dependencies)
			if openCount, err := beadQueryClient.CountByStatus("open"); err == nil {
				status.BlockedCount = openCount - status.ReadyBeadCount
				if status.BlockedCount < 0 {
					status.BlockedCount = 0
				}
			}

			// If startedAt is provided, populate "closed this run" count.
			if startedAt != nil {
				if count, err := beadQueryClient.CountClosedAfter(*startedAt); err == nil {
					status.ClosedThisRunCount = count
				}
			}
		}
	}
	// If client is nil, all counts remain zero

	// Generate recommendation based on priority
	status.Recommendation = generateRecommendation(status)

	return status, nil
}

// ReadStatus reads pipeline state from gromit data sources and returns structured status
func ReadStatus(gromitDir, specsDir, plansDir string, startedAt *time.Time) (*PipelineStatus, error) {
	// Create concrete implementations of required interfaces
	backlogFile, err := backlog.NewFile(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating backlog file: %w", err)
	}

	// Optionally create bead client if a beads repo is present
	var beadQueryClient BeadQueryClient
	repoRoot := filepath.Dir(gromitDir)
	if hasBeadsRepo(repoRoot) {
		client, err := bead.NewClient()
		if err == nil {
			client.Dir = repoRoot
			beadQueryClient = client
		}
	}

	// Use dependency-injected implementation
	return ReadStatusWithDeps(gromitDir, specsDir, plansDir, startedAt, backlogFile, beadQueryClient)
}

func hasBeadsRepo(repoRoot string) bool {
	info, err := os.Stat(filepath.Join(repoRoot, ".beads"))
	return err == nil && info.IsDir()
}

// findMarkdownFiles returns all .md files in a directory
func findMarkdownFiles(dir string) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}

	return files, nil
}

// listReadyBeads returns a list of ready bead IDs and the count
func listReadyBeads(client BeadQueryClient) ([]string, int) {
	if client == nil {
		return []string{}, 0
	}

	// Get ready bead IDs
	ids, err := client.ListReadyIDs()
	if err != nil {
		return []string{}, 0
	}

	return ids, len(ids)
}

// generateRecommendation returns the recommended next action based on pipeline state
func generateRecommendation(status *PipelineStatus) string {
	// Priority: unrefined > unplanned > undecomposed > ready beads
	if status.UnrefinedCount > 0 {
		if len(status.UnrefinedIdeas) > 0 {
			return fmt.Sprintf("Refine idea: %s", truncate(status.UnrefinedIdeas[0], 50))
		}
		return "Refine backlog ideas"
	}

	if len(status.UnplannedSpecs) > 0 {
		return fmt.Sprintf("Plan spec %q", status.UnplannedSpecs[0])
	}

	if len(status.UndecomposedPlans) > 0 {
		return fmt.Sprintf("Decompose plan %q", status.UndecomposedPlans[0])
	}

	if status.ReadyBeadCount > 0 {
		return fmt.Sprintf("Run %d ready bead(s)", status.ReadyBeadCount)
	}

	return "No work in pipeline"
}

// truncate truncates a string to maxLen runes, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if maxLen < 3 {
		maxLen = 3
	}

	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}
