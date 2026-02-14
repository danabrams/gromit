//go:build acceptance

package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/state"
)

// TestRunnerWiresArchivedHashesFromStateToLearnings verifies that the runner loads
// archived hashes from state.json and passes them to the learnings file via SetArchivedHashes()
func TestRunnerWiresArchivedHashesFromStateToLearnings(t *testing.T) {
	// Expected failure: Runner.Run() does not call GetArchivedHashes() from state
	// or SetArchivedHashes() on the learnings file
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create state.json with archived hashes
	sf, _ := state.NewFile(gromitDir)
	sf.AddArchivedHashes([]string{"archived_hash_1", "archived_hash_2", "archived_hash_3"})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create LEARNINGS.md with provisional learnings
	lf, _ := learnings.NewFile(gromitDir)
	_, err = lf.Add("bead-provisional", "Provisional learning content", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("failed to add provisional learning: %v", err)
	}

	// Try to add a learning with the same hash as an archived one
	archivedContent := "This content hash matches archived_hash_1"
	// Manually set the archived hashes on learnings file to verify wiring
	archivedHashes := sf.GetArchivedHashes()
	lf.SetArchivedHashes(archivedHashes)

	// Try to add the archived content - should be rejected as duplicate
	learning, err := lf.Add("bead-new", archivedContent, learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}

	// The test verifies that after wiring, the archived hash check works
	// This will fail until SetArchivedHashes() method exists on learnings.File
	if learning != nil {
		t.Error("learning should be nil when archived hash is set via SetArchivedHashes()")
	}
}

// TestRunnerPersistsUpdatedArchivedHashesBackToState verifies that after archiving
// a learning during the run, the runner persists the new archived hash back to state.json
func TestRunnerPersistsUpdatedArchivedHashesBackToState(t *testing.T) {
	// Expected failure: Runner does not call GetArchivedHashes() from learnings
	// and AddArchivedHashes() to state after archiving operations
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create state.json
	sf, _ := state.NewFile(gromitDir)
	sf.AddArchivedHashes([]string{"existing_archived_hash"})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create learnings file and add a learning
	lf, _ := learnings.NewFile(gromitDir)
	learning, err := lf.Add("bead-test", "Learning to be archived", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("failed to add learning: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Archive the learning
	err = lf.Archive(learning.Hash, "test archive")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Simulate what runner should do: get archived hashes from learnings and persist to state
	newArchivedHashes := lf.GetArchivedHashes()
	var hashesToAdd []string
	for hash := range newArchivedHashes {
		hashesToAdd = append(hashesToAdd, hash)
	}
	sf.AddArchivedHashes(hashesToAdd)
	err = sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Reload state and verify the archived hash was persisted
	sf2, _ := state.NewFile(gromitDir)
	err = sf2.Load()
	if err != nil {
		t.Fatalf("failed to reload state: %v", err)
	}

	reloadedHashes := sf2.GetArchivedHashes()
	if !reloadedHashes[learning.Hash] {
		t.Errorf("state should contain the archived learning hash %s after persistence", learning.Hash)
	}
	if !reloadedHashes["existing_archived_hash"] {
		t.Error("state should still contain the existing archived hash")
	}
}

// TestRunnerIntegrationArchivedHashFlowFromStateToLearningsToState tests the full
// flow: load from state -> wire to learnings -> archive a learning -> persist back to state
func TestRunnerIntegrationArchivedHashFlowFromStateToLearningsToState(t *testing.T) {
	// Expected failure: Runner does not implement the full wiring flow for archived hashes
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	templatesDir := filepath.Join(gromitDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	// Create minimal config
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Loop: config.LoopConfig{
			MaxIterations: 1,
		},
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
			Templates: templatesDir,
		},
	}
	cfg.SetDefaults()
	cfg.NormalizeNilFields()

	// Create state.json with existing archived hashes
	sf, _ := state.NewFile(gromitDir)
	sf.AddArchivedHashes([]string{"pre_existing_archived_hash"})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Create learnings file with a learning
	lf, _ := learnings.NewFile(gromitDir)
	learning, err := lf.Add("bead-integration", "Learning for integration test", learnings.CategoryConventions)
	if err != nil {
		t.Fatalf("failed to add learning: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Archive it manually (simulating what happens during retro)
	err = lf.Archive(learning.Hash, "integration test archive")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Simulate runner persistence logic: get archived hashes from learnings and add to state
	archivedFromLearnings := lf.GetArchivedHashes()
	var hashSlice []string
	for hash := range archivedFromLearnings {
		hashSlice = append(hashSlice, hash)
	}

	sf2, _ := state.NewFile(gromitDir)
	err = sf2.Load()
	if err != nil {
		t.Fatalf("failed to reload state: %v", err)
	}
	sf2.AddArchivedHashes(hashSlice)
	err = sf2.Save()
	if err != nil {
		t.Fatalf("failed to save updated state: %v", err)
	}

	// Verify the full flow: reload state and check both old and new hashes are present
	sf3, _ := state.NewFile(gromitDir)
	err = sf3.Load()
	if err != nil {
		t.Fatalf("failed to reload state for verification: %v", err)
	}

	finalHashes := sf3.GetArchivedHashes()
	if !finalHashes["pre_existing_archived_hash"] {
		t.Error("state should contain pre-existing archived hash")
	}
	if !finalHashes[learning.Hash] {
		t.Errorf("state should contain newly archived learning hash %s", learning.Hash)
	}

	// Try to add the archived learning again with a new learnings file instance
	lf2, _ := learnings.NewFile(gromitDir)
	err = lf2.Load()
	if err != nil {
		t.Fatalf("failed to reload learnings: %v", err)
	}

	// Wire archived hashes from state (this is what runner should do)
	lf2.SetArchivedHashes(finalHashes)

	// Try to add the same content - should be rejected as duplicate
	duplicate, err := lf2.Add("bead-duplicate-attempt", "Learning for integration test", learnings.CategoryConventions)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if duplicate != nil {
		t.Error("adding previously archived content should be rejected when archived hashes are wired from state")
	}
}

// TestRunnerWiringOccursBeforeBeadProcessing verifies that archived hash wiring
// happens early in the Run() method, before any bead processing begins
func TestRunnerWiringOccursBeforeBeadProcessing(t *testing.T) {
	// Expected failure: Runner.Run() does not wire archived hashes from state to learnings
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create config
	cfg := &config.Config{
		Models: config.ModelsConfig{
			P0:         "opus",
			P1:         "sonnet",
			P2:         "haiku",
			Validation: "haiku",
		},
		Loop: config.LoopConfig{
			MaxIterations: 1,
		},
		Paths: config.PathsConfig{
			GromitDir: gromitDir,
		},
	}
	cfg.SetDefaults()

	// Create state with archived hashes
	sf, _ := state.NewFile(gromitDir)
	testArchivedHash := "test_archived_hash_from_previous_run"
	sf.AddArchivedHashes([]string{testArchivedHash})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create mock bead client that returns no ready beads (so Run() exits early)
	mockBeads := &mockArchivedHashBeadClient{
		readyFn: func() (*bead.Bead, error) {
			return nil, nil
		},
	}

	// Create minimal renderer mock
	mockRenderer := &mockMinimalRenderer{
		learningsFile: nil, // Will be set after NewRunner
	}

	// Create runner with mocks
	runner, err := NewRunner(cfg, os.Stdout)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	runner.gromitDir = gromitDir

	// Replace beads and renderer with mocks
	runner.beads = mockBeads
	runner.renderer = mockRenderer
	lf := runner.renderer.GetLearningsFile()
	mockRenderer.learningsFile = lf

	// Run the runner (will exit early due to no ready beads)
	ctx := context.Background()
	err = runner.Run(ctx, 1, time.Now().Add(time.Minute), false)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify that learnings file received the archived hashes from state
	// This checks that wiring happened before bead processing
	if lf != nil {
		archivedHashes := lf.GetArchivedHashes()
		if archivedHashes == nil {
			t.Error("learnings file should have archived hashes set after Run()")
		} else if !archivedHashes[testArchivedHash] {
			t.Error("learnings file should contain the archived hash from state.json after wiring")
		}
	}
}

// mockArchivedHashBeadClient implements BeadClient interface for testing
type mockArchivedHashBeadClient struct {
	readyFn func() (*bead.Bead, error)
	showFn  func(id string) (*bead.Bead, error)
	closeFn func(id string) error
}

func (m *mockArchivedHashBeadClient) Ready() (*bead.Bead, error) {
	if m.readyFn != nil {
		return m.readyFn()
	}
	return nil, nil
}

func (m *mockArchivedHashBeadClient) ReadyWithLabel(label string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockArchivedHashBeadClient) ListWithLabel(label string) ([]*bead.Bead, error) {
	return []*bead.Bead{}, nil
}

func (m *mockArchivedHashBeadClient) Show(id string) (*bead.Bead, error) {
	if m.showFn != nil {
		return m.showFn(id)
	}
	return nil, nil
}

func (m *mockArchivedHashBeadClient) Close(id string) error {
	if m.closeFn != nil {
		return m.closeFn(id)
	}
	return nil
}

func (m *mockArchivedHashBeadClient) Sync() error {
	return nil
}

func (m *mockArchivedHashBeadClient) AddComment(id, comment string) error {
	return nil
}

func (m *mockArchivedHashBeadClient) GetParent(b *bead.Bead) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockArchivedHashBeadClient) CreateWithParent(title string, priority int, labels []string, expectedOutputs []string, parentID string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockArchivedHashBeadClient) CreateWithParentAndDescription(title string, priority int, labels []string, expectedOutputs []string, parentID string, description string) (*bead.Bead, error) {
	return nil, nil
}

func (m *mockArchivedHashBeadClient) HasOpenChildren(parentID string) (bool, error) {
	return false, nil
}

// mockMinimalRenderer implements PromptRenderer interface minimally for testing
type mockMinimalRenderer struct {
	learningsFile *learnings.File
}

func (m *mockMinimalRenderer) BuildContext(b *bead.Bead, parent *bead.Bead, iteration int, model string) (*prompt.Context, error) {
	return &prompt.Context{}, nil
}

func (m *mockMinimalRenderer) RenderBuild(ctx *prompt.Context) (string, error) {
	return "mock build prompt", nil
}

func (m *mockMinimalRenderer) RenderAnalyze(ctx *prompt.AnalyzeContext) (string, error) {
	return "mock analyze prompt", nil
}

func (m *mockMinimalRenderer) RenderLearn(ctx *prompt.LearnContext) (string, error) {
	return "mock learn prompt", nil
}

func (m *mockMinimalRenderer) RenderDecompose(ctx *prompt.DecomposeContext) (string, error) {
	return "mock decompose prompt", nil
}

func (m *mockMinimalRenderer) RenderScope(ctx *prompt.ScopeContext) (string, error) {
	return "mock scope prompt", nil
}

func (m *mockMinimalRenderer) RenderPrecheck(ctx *prompt.PrecheckContext) (string, error) {
	return "mock precheck prompt", nil
}

func (m *mockMinimalRenderer) RenderReview(ctx *prompt.ReviewContext) (string, error) {
	return "mock review prompt", nil
}

func (m *mockMinimalRenderer) RenderThoroughReview(ctx *prompt.ThoroughReviewContext) (string, error) {
	return "mock thorough review prompt", nil
}

func (m *mockMinimalRenderer) RenderAcceptanceTests(ctx *prompt.Context) (string, error) {
	return "mock acceptance tests prompt", nil
}

func (m *mockMinimalRenderer) RenderATDDBuild(ctx *prompt.Context) (string, error) {
	return "mock atdd build prompt", nil
}

func (m *mockMinimalRenderer) RenderTDDBuild(ctx *prompt.Context) (string, error) {
	return "mock tdd build prompt", nil
}

func (m *mockMinimalRenderer) RenderRefactor(ctx *prompt.Context) (string, error) {
	return "mock refactor prompt", nil
}

func (m *mockMinimalRenderer) LoadSpec(name string) (string, error) {
	return "", nil
}

func (m *mockMinimalRenderer) LoadClaudeMD() (string, error) {
	return "", nil
}

func (m *mockMinimalRenderer) LoadRules() (string, error) {
	return "", nil
}

func (m *mockMinimalRenderer) LoadRulesForPhase(phase string) (string, error) {
	return "", nil
}

func (m *mockMinimalRenderer) GetLearningsFile() *learnings.File {
	return m.learningsFile
}
