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

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/loop"
	v2spec "github.com/danabrams/gromit/internal/v2/spec"
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
