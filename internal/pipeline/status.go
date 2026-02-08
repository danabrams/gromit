package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danabrams/gromit/internal/backlog"
	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/frontmatter"
)

// PipelineStatus represents the current state of the gromit pipeline
type PipelineStatus struct {
	UnrefinedCount    int      // Number of backlog items not yet refined
	UnrefinedIdeas    []string // Text of unrefined ideas
	UnplannedSpecs    []string // Names of specs without corresponding plans
	UndecomposedPlans []string // Names of plans not yet decomposed
	ReadyBeadCount    int      // Number of ready beads
	ReadyBeads        []string // IDs of ready beads (up to 3 shown, rest summarized)
	Recommendation    string   // Suggested next action
}

// ReadStatus reads pipeline state from gromit data sources and returns structured status
func ReadStatus(gromitDir, specsDir, plansDir string) (*PipelineStatus, error) {
	status := &PipelineStatus{
		UnrefinedIdeas:    []string{},
		UnplannedSpecs:    []string{},
		UndecomposedPlans: []string{},
	}

	// Read backlog for unrefined ideas
	backlogFile, err := backlog.NewFile(gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating backlog file: %w", err)
	}

	ideas, err := backlogFile.List()
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

	// Count ready beads and get their IDs (best-effort - handle bd not being available)
	// Create a client with working directory set to parent of gromitDir
	client, err := bead.NewClient()
	if err == nil {
		client.Dir = filepath.Dir(gromitDir)
		status.ReadyBeads, status.ReadyBeadCount = listReadyBeads(client)
	}
	// If client creation fails, ReadyBeadCount and ReadyBeads remain empty

	// Generate recommendation based on priority
	status.Recommendation = generateRecommendation(status)

	return status, nil
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
func listReadyBeads(client *bead.Client) ([]string, int) {
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
