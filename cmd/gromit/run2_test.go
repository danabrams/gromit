package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/event"
	"github.com/danabrams/gromit/internal/v2/loop"
	"github.com/danabrams/gromit/internal/v2/routing"
	v2spec "github.com/danabrams/gromit/internal/v2/spec"
	"github.com/danabrams/gromit/internal/v2/trackertypes"
)

func TestRun2FailsWhenDependenciesBlocked(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	writeSpecFile(t, specsDir, "prereq", map[string]string{"id": "prereq"})
	childSpec := writeSpecFile(t, specsDir, "child", map[string]string{
		"id":         "child",
		"depends_on": "prereq",
	})

	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	stubLoop := newSpecLoopFn
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		return &fakeSpecLoop{}, nil
	}
	defer func() { newSpecLoopFn = stubLoop }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	err := run2Cmd.RunE(run2Cmd, []string{childSpec})
	if err == nil {
		t.Fatal("expected dependency error")
	}
	var depErr *v2spec.SpecDependenciesError
	if !errors.As(err, &depErr) {
		t.Fatalf("expected SpecDependenciesError, got %v", err)
	}
}

func TestRun2InvokesSpecLoopWhenReady(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	writeSpecFile(t, specsDir, "dep", map[string]string{"id": "dep", "accepted": "true"})
	readySpec := writeSpecFile(t, specsDir, "ready-child", map[string]string{
		"id":         "ready-child",
		"depends_on": "dep",
	})

	var stubLoop fakeSpecLoop
	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	stub := newSpecLoopFn
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		return &stubLoop, nil
	}
	defer func() { newSpecLoopFn = stub }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2Cmd.RunE(run2Cmd, []string{readySpec}); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
	if !stubLoop.called {
		t.Fatal("expected spec loop to run")
	}
	if stubLoop.runSpecID != "ready-child" {
		t.Fatalf("run spec id = %q, want %q", stubLoop.runSpecID, "ready-child")
	}
}

func TestRun2WiresEmitterToSpecLoop(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	readySpec := writeSpecFile(t, specsDir, "ready-emitter", map[string]string{"id": "ready-emitter", "accepted": "true"})

	var subscriberEmitter *events.Emitter
	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		subscriberEmitter = emitter
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	var specLoopEmitter *events.Emitter
	stubEmitterFn := newSpecLoopEmitterFn
	newSpecLoopEmitterFn = func(emitter *events.Emitter) loop.SpecLoopOption {
		specLoopEmitter = emitter
		return loop.WithEmitter(emitter)
	}
	defer func() { newSpecLoopEmitterFn = stubEmitterFn }()

	stubLoop := newSpecLoopFn
	var stub fakeSpecLoop
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		return &stub, nil
	}
	defer func() { newSpecLoopFn = stubLoop }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2Cmd.RunE(run2Cmd, []string{readySpec}); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if !stub.called {
		t.Fatal("expected spec loop to run")
	}
	if subscriberEmitter == nil {
		t.Fatal("startRun2Subscribers never received emitter")
	}
	if specLoopEmitter == nil {
		t.Fatal("spec loop emitter option never invoked")
	}
	if subscriberEmitter != specLoopEmitter {
		t.Fatalf("emitter mismatch: subscriber=%p specloop=%p", subscriberEmitter, specLoopEmitter)
	}
}

func TestRun2WiresStageOptions(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	readySpec := writeSpecFile(t, specsDir, "ready-stage", map[string]string{"id": "ready-stage", "accepted": "true"})

	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	stubLoop := newSpecLoopFn
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		specLoopPtr := &loop.SpecLoop{}
		for _, opt := range opts {
			opt(specLoopPtr)
		}
		if fieldIsNil(specLoopPtr, "decomposeStage") {
			t.Fatalf("expected decompose stage option, got nil")
		}
		if fieldIsNil(specLoopPtr, "beadRunner") {
			t.Fatalf("expected bead loop option, got nil")
		}
		return &fakeSpecLoop{}, nil
	}
	defer func() { newSpecLoopFn = stubLoop }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2Cmd.RunE(run2Cmd, []string{readySpec}); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
}

func TestRun2WiresRemediationRunner(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	readySpec := writeSpecFile(t, specsDir, "ready-remediation", map[string]string{"id": "ready-remediation", "accepted": "true"})

	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	stubLoop := newSpecLoopFn
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		specLoopPtr := &loop.SpecLoop{}
		for _, opt := range opts {
			opt(specLoopPtr)
		}
		if fieldIsNil(specLoopPtr, "remediationRunner") {
			t.Fatalf("expected remediation runner option to run")
		}
		return &fakeSpecLoop{}, nil
	}
	defer func() { newSpecLoopFn = stubLoop }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2Cmd.RunE(run2Cmd, []string{readySpec}); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
}

func TestRun2WiresStageCommitterAndTypedEmitter(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	readySpec := writeSpecFile(t, specsDir, "ready-committer", map[string]string{"id": "ready-committer", "accepted": "true"})

	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	stubLoop := newSpecLoopFn
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		specLoopPtr := &loop.SpecLoop{}
		for _, opt := range opts {
			opt(specLoopPtr)
		}
		if fieldIsNil(specLoopPtr, "stageCommitter") {
			t.Fatalf("expected stageCommitter option, got nil")
		}
		if fieldIsNil(specLoopPtr, "typedEmitter") {
			t.Fatalf("expected typedEmitter option, got nil")
		}
		return &fakeSpecLoop{}, nil
	}
	defer func() { newSpecLoopFn = stubLoop }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2Cmd.RunE(run2Cmd, []string{readySpec}); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}
}

func TestRun2EpicFlagRunsAllSpecs(t *testing.T) {
	specsDir, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	alpha := writeSpecFile(t, specsDir, "alpha", map[string]string{"id": "alpha", "epic": "suite"})
	beta := writeSpecFile(t, specsDir, "beta", map[string]string{"id": "beta", "epic": "suite"})
	_ = beta
	writeSpecFile(t, specsDir, "other", map[string]string{"id": "other", "epic": "different"})

	if err := run2Cmd.Flags().Set("epic", "suite"); err != nil {
		t.Fatalf("setting epic flag: %v", err)
	}
	defer func() {
		if err := run2Cmd.Flags().Set("epic", ""); err != nil {
			t.Fatalf("resetting epic flag: %v", err)
		}
	}()

	stubSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = stubSubscribers }()

	var executed []string
	stubLoop := newSpecLoopFn
	newSpecLoopFn = func(adapters adapter.AdapterSet, cfg *config.Config, gate loop.DependencyGate, opts ...loop.SpecLoopOption) (specLoop, error) {
		return &recordingSpecLoop{recorded: &executed}, nil
	}
	defer func() { newSpecLoopFn = stubLoop }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2Cmd.RunE(run2Cmd, []string{alpha}); err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	if len(executed) != 2 {
		t.Fatalf("executed specs = %d, want %d", len(executed), 2)
	}
	if executed[0] != "alpha" || executed[1] != "beta" {
		t.Fatalf("executed order = %v, want [alpha beta]", executed)
	}
}

func TestRun2FromReviewUsesBeadLoop(t *testing.T) {
	_, cleanup := setupRun2TestEnv(t)
	defer cleanup()

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	fakeTracker := &fakeTaskTracker{
		response: []trackertypes.Bead{
			{
				ID:        "from-review-1",
				Title:     "review work",
				Labels:    []string{"from-review"},
				Priority:  1,
				DependsOn: []string{"dep-a"},
				BlockedBy: []string{"blocked"},
			},
		},
	}

	origTrackerFn := newTaskTrackerAdapterFn
	newTaskTrackerAdapterFn = func(_ *bead.Client) trackertypes.TaskTracker { return fakeTracker }
	defer func() { newTaskTrackerAdapterFn = origTrackerFn }()

	var captured struct {
		called bool
		beads  []*bead.Bead
	}

	origRunBeadLoopFn := runBeadLoopFn
	runBeadLoopFn = func(beadLoop *loop.BeadLoop, ctx context.Context, beads []*bead.Bead, stopCh <-chan struct{}) (loop.BeadLoopResult, error) {
		captured.called = true
		captured.beads = append([]*bead.Bead(nil), beads...)
		return loop.BeadLoopResult{}, nil
	}
	defer func() { runBeadLoopFn = origRunBeadLoopFn }()

	origComponentsFn := newRun2LoopComponentsFn
	newRun2LoopComponentsFn = func(cfg *config.Config, adapters adapter.AdapterSet, legacyEmitter *events.Emitter, output io.Writer, router *routing.Router, phaseModels map[string]string) (*loop.Run2LoopComponents, error) {
		emitter := events.NewEmitter()
		return &loop.Run2LoopComponents{
			BeadLoop:     &loop.BeadLoop{},
			Emitter:      emitter,
			TypedEmitter: event.NewEmitter(),
		}, nil
	}
	defer func() { newRun2LoopComponentsFn = origComponentsFn }()

	origSubscribers := startRun2SubscribersFn
	startRun2SubscribersFn = func(ctx context.Context, emitter *events.Emitter, output io.Writer, logsDir string) (*sync.WaitGroup, error) {
		return &sync.WaitGroup{}, nil
	}
	defer func() { startRun2SubscribersFn = origSubscribers }()

	run2Cmd.SetOut(io.Discard)
	run2Cmd.SetErr(io.Discard)

	if err := run2FromReview(run2Cmd, cfg); err != nil {
		t.Fatalf("run2FromReview = %v", err)
	}

	if !captured.called {
		t.Fatal("bead loop never executed")
	}
	if got := fakeTracker.lastQuery.Labels; !reflect.DeepEqual(got, []string{"from-review"}) {
		t.Fatalf("tracker labels = %v, want %v", got, []string{"from-review"})
	}
	if fakeTracker.lastQuery.Status != "open" {
		t.Fatalf("tracker status = %q, want %q", fakeTracker.lastQuery.Status, "open")
	}
	if len(captured.beads) != len(fakeTracker.response) {
		t.Fatalf("beads = %d, want %d", len(captured.beads), len(fakeTracker.response))
	}
	got := captured.beads[0]
	if len(got.DependsOn) != 1 || got.DependsOn[0].ID != "dep-a" {
		t.Fatalf("depends_on = %v, want dep-a", got.DependsOn)
	}
	if len(got.BlockedBy) != 1 || got.BlockedBy[0].ID != "blocked" {
		t.Fatalf("blocked_by = %v, want blocked", got.BlockedBy)
	}
}

func setupRun2TestEnv(t *testing.T) (string, func()) {
	t.Helper()

	tempDir := t.TempDir()
	gromitDir := filepath.Join(tempDir, ".gromit")
	specsDir := filepath.Join(gromitDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("setup specs dir: %v", err)
	}

	cfgPath := filepath.Join(tempDir, "gromit.yaml")
	if err := os.WriteFile(cfgPath, []byte(configForProfile("go")), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origConfigPath := configPath
	origLogsFn := resolveMainRepoLogsDirFn
	origWD, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	configPath = cfgPath
	resolveMainRepoLogsDirFn = func(_ string) string {
		return filepath.Join(gromitDir, "logs")
	}

	cleanup := func() {
		configPath = origConfigPath
		resolveMainRepoLogsDirFn = origLogsFn
		os.Chdir(origWD)
	}

	return specsDir, cleanup
}

func writeSpecFile(t *testing.T, specsDir, id string, frontmatter map[string]string) string {
	t.Helper()

	path := filepath.Join(specsDir, id+".md")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create spec: %v", err)
	}
	defer f.Close()

	if len(frontmatter) > 0 {
		f.WriteString("---\n")
		for key, value := range frontmatter {
			f.WriteString(key)
			f.WriteString(": ")
			f.WriteString(value)
			f.WriteString("\n")
		}
		f.WriteString("---\n")
	}
	f.WriteString("# spec body\n")
	return path
}

var _ specLoop = (*fakeSpecLoop)(nil)

type fakeSpecLoop struct {
	called    bool
	runSpecID string
	stopCh    <-chan struct{}
}

func (f *fakeSpecLoop) Run(ctx context.Context, specID string, stopCh <-chan struct{}) error {
	f.called = true
	f.runSpecID = specID
	f.stopCh = stopCh
	return nil
}

type recordingSpecLoop struct {
	recorded *[]string
}

func (r *recordingSpecLoop) Run(ctx context.Context, specID string, stopCh <-chan struct{}) error {
	if r == nil || r.recorded == nil {
		return nil
	}
	*r.recorded = append(*r.recorded, specID)
	return nil
}

type fakeTaskTracker struct {
	lastQuery trackertypes.TaskTrackerQueryBeadsRequest
	response  []trackertypes.Bead
}

func (f *fakeTaskTracker) NextBead(context.Context, trackertypes.TaskTrackerNextBeadRequest) (*trackertypes.TaskTrackerNextBeadResponse, error) {
	return nil, nil
}

func (f *fakeTaskTracker) ShowBead(context.Context, string) (*trackertypes.Bead, error) {
	return nil, nil
}

func (f *fakeTaskTracker) CreateBead(context.Context, trackertypes.TaskTrackerCreateBeadRequest) (*trackertypes.TaskTrackerCreateBeadResponse, error) {
	return nil, nil
}

func (f *fakeTaskTracker) CloseBead(context.Context, trackertypes.TaskTrackerCloseBeadRequest) (*trackertypes.TaskTrackerCloseBeadResponse, error) {
	return nil, nil
}

func (f *fakeTaskTracker) QueryBeads(_ context.Context, req trackertypes.TaskTrackerQueryBeadsRequest) (*trackertypes.TaskTrackerQueryBeadsResponse, error) {
	f.lastQuery = trackertypes.TaskTrackerQueryBeadsRequest{
		Labels: append([]string(nil), req.Labels...),
		Status: req.Status,
	}
	resp := &trackertypes.TaskTrackerQueryBeadsResponse{}
	if len(f.response) > 0 {
		resp.Beads = append([]trackertypes.Bead(nil), f.response...)
	}
	return resp, nil
}

func fieldIsNil(specLoop *loop.SpecLoop, fieldName string) bool {
	value := reflect.ValueOf(specLoop).Elem().FieldByName(fieldName)
	if !value.IsValid() {
		return true
	}
	value = reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem()
	if !value.IsValid() {
		return true
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Ptr, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan:
		return value.IsNil()
	default:
		return false
	}
}

func TestIsCodexBinary(t *testing.T) {
	tests := []struct {
		binary string
		want   bool
	}{
		{"codex", true},
		{"/usr/local/bin/codex", true},
		{"claude", false},
		{"/usr/local/bin/claude", false},
		{"my-codex-wrapper", true},
		{"Codex", true},
	}
	for _, tt := range tests {
		t.Run(tt.binary, func(t *testing.T) {
			got := isCodexBinary(tt.binary)
			if got != tt.want {
				t.Errorf("isCodexBinary(%q) = %v, want %v", tt.binary, got, tt.want)
			}
		})
	}
}

func TestFromReviewLabels(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want []string
	}{
		{name: "no spec", spec: "", want: []string{"from-review"}},
		{name: "with spec", spec: "stable", want: []string{"from-review", "spec:stable"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fromReviewLabels(tt.spec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("labels = %v, want %v", got, tt.want)
			}
		})
	}
}
