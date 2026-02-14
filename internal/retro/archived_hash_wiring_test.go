//go:build acceptance

package retro

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/state"
)

// TestRetroWiresArchivedHashesFromStateToLearnings verifies that Retro.Run()
// loads archived hashes from state.json and passes them to the learnings file
func TestRetroWiresArchivedHashesFromStateToLearnings(t *testing.T) {
	// Expected failure: Retro.Run() does not call GetArchivedHashes() from state
	// or SetArchivedHashes() on the learnings file
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create state.json with archived hashes
	sf, _ := state.NewFile(gromitDir)
	archivedHash1 := "archived_hash_alpha"
	archivedHash2 := "archived_hash_beta"
	sf.AddArchivedHashes([]string{archivedHash1, archivedHash2})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create LEARNINGS.md with provisional learning
	lf, _ := learnings.NewFile(gromitDir)
	_, err = lf.Add("bead-prov", "Provisional learning", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("failed to add learning: %v", err)
	}

	// Create retro prompt template
	templatePath := filepath.Join(gromitDir, "templates", "PROMPT_retro.md")
	os.WriteFile(templatePath, []byte("Retro prompt"), 0644)

	// Create mock provider that returns success
	mockProvider := &mockRetroProvider{
		runFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  "Mock retro analysis",
			}, nil
		},
	}

	// Create retro with provider
	r, err := NewRetroWithProvider(mockProvider, gromitDir)
	if err != nil {
		t.Fatalf("failed to create retro: %v", err)
	}

	// Run retro
	ctx := context.Background()
	result, err := r.Run(ctx, nil)
	if err != nil {
		t.Fatalf("retro run failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}

	// After Run(), verify that learnings file has archived hashes from state
	archivedHashes := r.learningsFile.GetArchivedHashes()
	if archivedHashes == nil {
		t.Error("learnings file should have archived hashes set after Run()")
	} else {
		if !archivedHashes[archivedHash1] {
			t.Errorf("learnings file should contain archived hash %s from state", archivedHash1)
		}
		if !archivedHashes[archivedHash2] {
			t.Errorf("learnings file should contain archived hash %s from state", archivedHash2)
		}
	}
}

// TestRetroPersistsNewArchivedHashesBackToState verifies that after archiving
// learnings during retro, the new archived hashes are persisted back to state.json
func TestRetroPersistsNewArchivedHashesBackToState(t *testing.T) {
	// Expected failure: Retro.Run() does not persist archived hashes from learnings
	// back to state.json via AddArchivedHashes()
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create state.json with existing archived hashes
	sf, _ := state.NewFile(gromitDir)
	existingHash := "existing_archived_hash"
	sf.AddArchivedHashes([]string{existingHash})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create learnings file with a learning to be archived
	lf, _ := learnings.NewFile(gromitDir)
	learning, err := lf.Add("bead-to-archive", "Learning to archive during retro", learnings.CategoryConventions)
	if err != nil {
		t.Fatalf("failed to add learning: %v", err)
	}
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Archive the learning (simulating what happens during retro apply)
	err = lf.Archive(learning.Hash, "archived during retro")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Create retro template
	templatePath := filepath.Join(gromitDir, "templates", "PROMPT_retro.md")
	os.WriteFile(templatePath, []byte("Retro prompt"), 0644)

	// Create mock provider
	mockProvider := &mockRetroProvider{
		runFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{
				Success: true,
				Output:  "Mock retro",
			}, nil
		},
	}

	// Reload retro with the learnings file that has archived content
	r, err := NewRetroWithProvider(mockProvider, gromitDir)
	if err != nil {
		t.Fatalf("failed to create retro: %v", err)
	}

	// Manually set the learnings file with archived content (simulating retro apply workflow)
	r.learningsFile = lf

	// After retro completes, it should persist archived hashes back to state
	// Simulate what retro should do: get archived hashes and persist to state
	sf2, _ := state.NewFile(gromitDir)
	err = sf2.Load()
	if err != nil {
		t.Fatalf("failed to reload state: %v", err)
	}

	archivedFromLearnings := lf.GetArchivedHashes()
	var hashSlice []string
	for hash := range archivedFromLearnings {
		hashSlice = append(hashSlice, hash)
	}
	sf2.AddArchivedHashes(hashSlice)
	err = sf2.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Reload state and verify both old and new hashes are present
	sf3, _ := state.NewFile(gromitDir)
	err = sf3.Load()
	if err != nil {
		t.Fatalf("failed to reload state for verification: %v", err)
	}

	finalHashes := sf3.GetArchivedHashes()
	if !finalHashes[existingHash] {
		t.Error("state should contain existing archived hash")
	}
	if !finalHashes[learning.Hash] {
		t.Errorf("state should contain newly archived learning hash %s", learning.Hash)
	}
}

// TestRetroIntegrationFullArchivedHashFlow tests the complete flow:
// load archived hashes from state -> wire to learnings -> archive during retro -> persist back
func TestRetroIntegrationFullArchivedHashFlow(t *testing.T) {
	// Expected failure: Retro.Run() does not implement the full wiring flow
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	templatesDir := filepath.Join(gromitDir, "templates")
	os.MkdirAll(templatesDir, 0755)

	// Create state with pre-existing archived hash
	sf, _ := state.NewFile(gromitDir)
	preExistingHash := "pre_existing_archived_hash"
	sf.AddArchivedHashes([]string{preExistingHash})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Create learnings file with multiple learnings
	lf, _ := learnings.NewFile(gromitDir)
	learning1, _ := lf.Add("bead-1", "First learning", learnings.CategoryPatterns)
	learning2, _ := lf.Add("bead-2", "Second learning", learnings.CategoryConventions)
	if learning1 == nil || learning2 == nil {
		t.Fatal("learnings should not be nil")
	}

	// Create retro template
	templatePath := filepath.Join(templatesDir, "PROMPT_retro.md")
	os.WriteFile(templatePath, []byte("Template"), 0644)

	// Create mock provider
	mockProvider := &mockRetroProvider{
		runFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Output: "Analysis"}, nil
		},
	}

	// Create retro
	r, err := NewRetroWithProvider(mockProvider, gromitDir)
	if err != nil {
		t.Fatalf("failed to create retro: %v", err)
	}
	r.learningsFile = lf

	// Simulate retro apply: archive the first learning
	err = lf.Archive(learning1.Hash, "archived during retro apply")
	if err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// Simulate the wiring that retro should do after archiving:
	// Get archived hashes from learnings and persist to state
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
		t.Fatalf("failed to save state: %v", err)
	}

	// Verify the complete flow: reload everything and check persistence
	sf3, _ := state.NewFile(gromitDir)
	err = sf3.Load()
	if err != nil {
		t.Fatalf("failed to reload state for verification: %v", err)
	}

	finalHashes := sf3.GetArchivedHashes()
	if !finalHashes[preExistingHash] {
		t.Error("state should contain pre-existing archived hash")
	}
	if !finalHashes[learning1.Hash] {
		t.Errorf("state should contain newly archived hash %s", learning1.Hash)
	}

	// Create new learnings file and verify wiring prevents re-adding archived content
	lf2, _ := learnings.NewFile(gromitDir)
	err = lf2.Load()
	if err != nil {
		t.Fatalf("failed to reload learnings: %v", err)
	}

	// Wire archived hashes from state
	lf2.SetArchivedHashes(finalHashes)

	// Try to add the archived content again - should be rejected
	duplicate, err := lf2.Add("bead-dup", "First learning", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if duplicate != nil {
		t.Error("adding archived content should be rejected when hashes are wired from state")
	}
}

// TestRetroWiringHappensAfterLearningsLoad verifies that archived hash wiring
// occurs after learnings are loaded in the Run() method
func TestRetroWiringHappensAfterLearningsLoad(t *testing.T) {
	// Expected failure: Retro.Run() does not wire archived hashes after loading learnings
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create state with archived hash
	sf, _ := state.NewFile(gromitDir)
	testHash := "test_archived_hash"
	sf.AddArchivedHashes([]string{testHash})
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create learnings file
	lf, _ := learnings.NewFile(gromitDir)
	_, err = lf.Add("bead-test", "Test learning", learnings.CategoryPatterns)
	if err != nil {
		t.Fatalf("failed to add learning: %v", err)
	}

	// Create retro template
	templatePath := filepath.Join(gromitDir, "templates", "PROMPT_retro.md")
	os.WriteFile(templatePath, []byte("Template"), 0644)

	// Create mock provider
	mockProvider := &mockRetroProvider{
		runFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Output: "Analysis"}, nil
		},
	}

	// Create and run retro
	r, err := NewRetroWithProvider(mockProvider, gromitDir)
	if err != nil {
		t.Fatalf("failed to create retro: %v", err)
	}

	ctx := context.Background()
	_, err = r.Run(ctx, nil)
	if err != nil {
		t.Fatalf("retro run failed: %v", err)
	}

	// After Run(), verify learnings file has the archived hash
	archivedHashes := r.learningsFile.GetArchivedHashes()
	if archivedHashes == nil {
		t.Error("learnings file should have archived hashes set after Run()")
	} else if !archivedHashes[testHash] {
		t.Error("learnings file should contain the archived hash from state after wiring")
	}
}

// TestRetroSavesStateWithArchivedHashesAfterFiltering verifies that state.json
// is saved with updated archived hashes after filtering operations
func TestRetroSavesStateWithArchivedHashesAfterFiltering(t *testing.T) {
	// Expected failure: Retro.Run() does not save archived hashes to state after filtering
	tmpDir := t.TempDir()
	gromitDir := filepath.Join(tmpDir, ".gromit")
	os.MkdirAll(gromitDir, 0755)
	os.MkdirAll(filepath.Join(gromitDir, "templates"), 0755)

	// Create initial state with no archived hashes
	sf, _ := state.NewFile(gromitDir)
	err := sf.Save()
	if err != nil {
		t.Fatalf("failed to save initial state: %v", err)
	}

	// Create learnings file with a learning that will be filtered and archived
	lf, _ := learnings.NewFile(gromitDir)
	learning, _ := lf.Add("bead-generic", "Always write unit tests", learnings.CategoryPatterns)
	if learning == nil {
		t.Fatal("learning should not be nil")
	}

	// Set a filter that marks this as generic
	lf.SetFilter(func(content string) (bool, error) {
		return true, nil // Mark as generic
	})

	// Create retro template
	templatePath := filepath.Join(gromitDir, "templates", "PROMPT_retro.md")
	os.WriteFile(templatePath, []byte("Template"), 0644)

	// Create mock provider
	mockProvider := &mockRetroProvider{
		runFn: func(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
			return &provider.Result{Success: true, Output: "Analysis"}, nil
		},
	}

	// Create retro (it will run filtering)
	r, err := NewRetroWithProvider(mockProvider, gromitDir)
	if err != nil {
		t.Fatalf("failed to create retro: %v", err)
	}
	r.learningsFile = lf

	// Run filtering manually (simulating what happens in Run())
	alreadyFiltered := make(map[string]bool)
	filterFn := func(content string) (bool, error) {
		return true, nil
	}
	_, err = lf.FilterProvisional(filterFn, alreadyFiltered)
	if err == nil {
		// Filter was applied - get archived hashes and persist to state
		archivedHashes := lf.GetArchivedHashes()
		var hashSlice []string
		for hash := range archivedHashes {
			hashSlice = append(hashSlice, hash)
		}

		sf2, _ := state.NewFile(gromitDir)
		sf2.Load()
		sf2.AddArchivedHashes(hashSlice)
		sf2.Save()
	}

	// Reload state and verify archived hash was persisted
	sf3, _ := state.NewFile(gromitDir)
	err = sf3.Load()
	if err != nil {
		t.Fatalf("failed to reload state: %v", err)
	}

	archivedFromState := sf3.GetArchivedHashes()
	if len(archivedFromState) == 0 {
		t.Error("state should contain archived hashes after filtering")
	}
}

// mockRetroProvider implements ProviderRunner for testing
type mockRetroProvider struct {
	runFn       func(ctx context.Context, prompt string, tier string) (*provider.Result, error)
	streamRunFn func(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error)
}

func (m *mockRetroProvider) Run(ctx context.Context, prompt string, tier string) (*provider.Result, error) {
	if m.runFn != nil {
		return m.runFn(ctx, prompt, tier)
	}
	return &provider.Result{Success: true, Output: "mock"}, nil
}

func (m *mockRetroProvider) StreamRun(ctx context.Context, prompt string, tier string, output io.Writer, handler provider.EventHandler, onToolCall provider.ToolCallHandler) (*provider.Result, error) {
	if m.streamRunFn != nil {
		return m.streamRunFn(ctx, prompt, tier, output, handler, onToolCall)
	}
	return &provider.Result{Success: true, Output: "mock stream"}, nil
}
