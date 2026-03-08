package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/events"
	"github.com/danabrams/gromit/internal/events/cli"
	"github.com/danabrams/gromit/internal/events/eventtest"
	"github.com/danabrams/gromit/internal/events/stream"
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/event"
	stagepkg "github.com/danabrams/gromit/internal/v2/stage"
)

func TestIntegration_FileSubscriberPreservesLegacySubscriberFlow(t *testing.T) {
	t.Parallel()

	specID := "spec-subscriber-compat"
	cfg := &config.Config{}

	legacyEmitter := events.NewEmitter()
	typedEmitter := event.NewEmitter()

	var cliOutput bytes.Buffer
	streamOutput := &bytes.Buffer{}

	cliSubscriber := cli.NewCLISubscriber(cli.BasicWriter(&cliOutput), legacyEmitter)
	streamSubscriber := stream.NewWriterSubscriber(integrationNopWriteCloser{Writer: streamOutput}, legacyEmitter)

	subscriberCtx, cancelSubscribers := context.WithCancel(context.Background())
	var subscriberWG sync.WaitGroup
	subscriberWG.Add(2)
	go func() {
		defer subscriberWG.Done()
		_ = cliSubscriber.Start(subscriberCtx)
	}()
	go func() {
		defer subscriberWG.Done()
		_ = streamSubscriber.Start(subscriberCtx)
	}()

	t.Cleanup(func() {
		cancelSubscribers()
		typedEmitter.Close()
		legacyEmitter.Close()
		subscriberWG.Wait()
	})

	readyCtx, readyCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer readyCancel()
	if err := eventtest.WaitForSubscriberReady(readyCtx, legacyEmitter); err != nil {
		t.Fatalf("wait for legacy subscribers: %v", err)
	}

	legacyEmitter.Emit(&events.LogEvent{Level: "info", Message: "compat warmup"})
	if err := eventtest.WaitForCondition(readyCtx, func() bool {
		return strings.Contains(cliOutput.String(), "compat warmup") &&
			strings.Contains(streamOutput.String(), "\"type\":\"log\"")
	}); err != nil {
		t.Fatalf("warmup event did not reach legacy subscribers: %v", err)
	}

	beadLoop, err := NewBeadLoop(BeadLoopConfig{
		Gate:          newNoopStage("gate"),
		Build:         newNoopStage("build"),
		Validate:      newNoopStage("validate"),
		Review:        newNoopStage("review"),
		Epilogue:      newNoopStage("epilogue"),
		Emitter:       typedEmitter,
		LegacyEmitter: legacyEmitter,
	})
	if err != nil {
		t.Fatalf("create bead loop: %v", err)
	}

	git := newIntegrationGitAdapter(t)
	llmAdapter := newIntegrationLLMAdapter()
	taskTracker := newIntegrationTaskTrackerAdapter()
	presenter := newIntegrationPresenterAdapter(t)
	planStage := newFakePlanStage(specID)
	decompose := newFakeDecomposeStage(specID)
	accept := newScriptedAcceptStage(stagepkg.Result{Decision: stagepkg.DecisionProceed})
	presentStage, summaryCtx := newPresentStageForTest(t, cfg, presenter)

	loopInstance, err := NewSpecLoop(
		adapter.AdapterSet{
			Git:         git,
			LLM:         llmAdapter,
			TaskTracker: taskTracker,
			Presenter:   presenter,
		},
		cfg, noopDependencyGate{},
		WithEmitter(legacyEmitter),
		WithTypedEmitter(typedEmitter),
		WithPlanStage(planStage),
		WithPresentStage(presentStage, summaryCtx),
		WithAcceptStage(accept),
		WithDecomposeStage(decompose),
		WithBeadLoop(beadLoop),
	)
	if err != nil {
		t.Fatalf("create spec loop: %v", err)
	}

	if err := loopInstance.Run(context.Background(), specID, nil); err != nil {
		t.Fatalf("run spec loop: %v", err)
	}

	typedEmitter.Close()

	observeCtx, observeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer observeCancel()
	beadID := specID + "-bead"
	if err := eventtest.WaitForCondition(observeCtx, func() bool {
		return strings.Contains(cliOutput.String(), beadID) &&
			strings.Contains(streamOutput.String(), "\"type\":\"iteration_start\"")
	}); err != nil {
		t.Fatalf("bridged events did not reach legacy subscribers: %v", err)
	}

	eventsPath := filepath.Join(git.WorktreeRoot, specID, ".gromit", "v2", "events.jsonl")
	waitForEventsFile(t, eventsPath)

	typedEventTypes := readTypedEventTypeCounts(t, eventsPath)
	if typedEventTypes[event.EventTypeSpecStarted] == 0 {
		t.Fatalf("typed events missing %q: %+v", event.EventTypeSpecStarted, typedEventTypes)
	}
	if typedEventTypes[event.EventTypeBeadStarted] == 0 {
		t.Fatalf("typed events missing %q: %+v", event.EventTypeBeadStarted, typedEventTypes)
	}
	if typedEventTypes[event.EventTypeBuildInvocationStart] == 0 {
		t.Fatalf("typed events missing %q: %+v", event.EventTypeBuildInvocationStart, typedEventTypes)
	}

	legacyTypeCounts := readLegacyStreamTypeCounts(t, streamOutput.String())
	if legacyTypeCounts["spec_started"] == 0 {
		t.Fatalf("legacy stream missing spec_started: %+v", legacyTypeCounts)
	}
	if legacyTypeCounts["iteration_start"] == 0 {
		t.Fatalf("legacy stream missing iteration_start: %+v", legacyTypeCounts)
	}
	if legacyTypeCounts["build_start"] == 0 {
		t.Fatalf("legacy stream missing build_start: %+v", legacyTypeCounts)
	}
}

type integrationNopWriteCloser struct {
	io.Writer
}

func (integrationNopWriteCloser) Close() error { return nil }

func readTypedEventTypeCounts(t *testing.T, path string) map[string]int {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read typed events file: %v", err)
	}

	counts := map[string]int{}
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var evt struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			t.Fatalf("decode typed event line %d: %v", i, err)
		}
		counts[evt.Type]++
	}
	return counts
}

func readLegacyStreamTypeCounts(t *testing.T, raw string) map[string]int {
	t.Helper()

	counts := map[string]int{}
	for i, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode legacy stream line %d: %v", i, err)
		}
		counts[entry.Type]++
	}
	return counts
}
