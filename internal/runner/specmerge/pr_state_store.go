package specmerge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/danabrams/gromit/internal/review"
)

const (
	prStateDirPerm       os.FileMode = 0o755
	prStateFilePerm      os.FileMode = 0o644
	prStateSchemaVersion             = 1
)

type prStateFileStore struct {
	path string
	mu   sync.Mutex
}

type prStateFileSchema struct {
	SchemaVersion int        `json:"schema_version"`
	States        []*PRState `json:"states"`
}

// NewPRStateStoreFile returns a PRStateStore that persists state under the given gromit directory.
func NewPRStateStoreFile(gromitDir string) (PRStateStore, error) {
	dir := strings.TrimSpace(gromitDir)
	if dir == "" {
		return nil, fmt.Errorf("gromit dir is required")
	}
	return &prStateFileStore{
		path: filepath.Join(dir, "spec-pr-state.json"),
	}, nil
}

func (f *prStateFileStore) List(_ context.Context) ([]*PRState, error) {
	if f == nil {
		return nil, fmt.Errorf("pr state store is nil")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	var states []*PRState
	if err := f.withFileLock(func() error {
		var err error
		states, err = f.readStates()
		return err
	}); err != nil {
		return nil, err
	}

	return clonePRStates(states), nil
}

func (f *prStateFileStore) Save(_ context.Context, state *PRState) error {
	if f == nil {
		return fmt.Errorf("pr state store is nil")
	}
	if state == nil {
		return fmt.Errorf("state is nil")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	return f.withFileLock(func() error {
		states, err := f.readStates()
		if err != nil {
			return err
		}
		updated := upsertState(states, state)
		return f.writeStates(updated)
	})
}

func (f *prStateFileStore) readStates() ([]*PRState, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading pr state file: %w", err)
	}

	var schema prStateFileSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("parsing pr state file: %w", err)
	}

	return clonePRStates(schema.States), nil
}

func (f *prStateFileStore) writeStates(states []*PRState) error {
	if err := os.MkdirAll(filepath.Dir(f.path), prStateDirPerm); err != nil {
		return fmt.Errorf("creating pr state directory: %w", err)
	}

	payload := prStateFileSchema{
		SchemaVersion: prStateSchemaVersion,
		States:        clonePRStates(states),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling pr state file: %w", err)
	}

	return os.WriteFile(f.path, data, prStateFilePerm)
}

func (f *prStateFileStore) withFileLock(fn func() error) error {
	lockPath := f.path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, prStateFilePerm)
	if err != nil {
		return fmt.Errorf("opening pr state lock file: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring pr state lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return fn()
}

func upsertState(states []*PRState, state *PRState) []*PRState {
	if len(states) == 0 {
		return []*PRState{clonePRState(state)}
	}
	key := stateKey(state)
	if key == "" {
		return append(states, clonePRState(state))
	}
	for i, existing := range states {
		if stateKey(existing) == key {
			states[i] = clonePRState(state)
			return states
		}
	}
	return append(states, clonePRState(state))
}

func stateKey(state *PRState) string {
	if state == nil {
		return ""
	}
	spec := strings.TrimSpace(state.SpecName)
	var builder strings.Builder
	if spec != "" {
		builder.WriteString(spec)
	}
	if state.PRRef.Number > 0 {
		if builder.Len() > 0 {
			builder.WriteRune('#')
		}
		builder.WriteString(strconv.Itoa(state.PRRef.Number))
	}
	return builder.String()
}

func clonePRStates(states []*PRState) []*PRState {
	if len(states) == 0 {
		return nil
	}
	copies := make([]*PRState, len(states))
	for i, state := range states {
		copies[i] = clonePRState(state)
	}
	return copies
}

func clonePRState(state *PRState) *PRState {
	if state == nil {
		return nil
	}
	copied := *state
	copied.LastChecks = cloneCheckStatuses(state.LastChecks)
	copied.StageResults = cloneStageResults(state.StageResults)
	return &copied
}

func cloneCheckStatuses(checks []CheckStatus) []CheckStatus {
	if len(checks) == 0 {
		return nil
	}
	clones := make([]CheckStatus, len(checks))
	copy(clones, checks)
	return clones
}

func cloneStageResults(results []StageResult) []StageResult {
	if len(results) == 0 {
		return nil
	}
	clones := make([]StageResult, len(results))
	for i, result := range results {
		clones[i] = cloneStageResult(result)
	}
	return clones
}

func cloneStageResult(result StageResult) StageResult {
	out := result
	out.ReviewResult = cloneReviewResult(result.ReviewResult)
	if result.ProviderResult != nil {
		pr := *result.ProviderResult
		out.ProviderResult = &pr
	}
	return out
}

func cloneReviewResult(result *review.ReviewResult) *review.ReviewResult {
	if result == nil {
		return nil
	}
	copied := *result
	copied.FixesApplied = cloneStringSlice(result.FixesApplied)
	copied.FixCategories = cloneStringSlice(result.FixCategories)
	copied.Learnings = cloneStringSlice(result.Learnings)
	copied.BeadsToCreate = cloneBeadProposals(result.BeadsToCreate)
	copied.BacklogItems = cloneBacklogItems(result.BacklogItems)
	return &copied
}

func cloneBeadProposals(items []review.BeadProposal) []review.BeadProposal {
	if len(items) == 0 {
		return nil
	}
	copies := make([]review.BeadProposal, len(items))
	for i, item := range items {
		clones := item
		clones.Labels = cloneStringSlice(item.Labels)
		clones.ExpectedOutputs = cloneStringSlice(item.ExpectedOutputs)
		copies[i] = clones
	}
	return copies
}

func cloneBacklogItems(items []review.BacklogItem) []review.BacklogItem {
	if len(items) == 0 {
		return nil
	}
	copies := make([]review.BacklogItem, len(items))
	for i, item := range items {
		clones := item
		clones.ExpectedOutputs = cloneStringSlice(item.ExpectedOutputs)
		copies[i] = clones
	}
	return copies
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
