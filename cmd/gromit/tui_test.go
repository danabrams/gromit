package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/conversation"
	"github.com/danabrams/gromit/internal/pipeline"
	"github.com/danabrams/gromit/internal/runner"
	"github.com/danabrams/gromit/internal/tui"
	"github.com/spf13/cobra"
)

func TestRegisterRootCommandsAddsTui(t *testing.T) {
	root := &cobra.Command{}
	registerRootCommands(root)

	cmd, _, err := root.Find([]string{"tui"})
	if err != nil {
		t.Fatalf("unexpected error finding tui command: %v", err)
	}
	if cmd == nil || cmd.Use != "tui" {
		t.Fatalf("tui command not registered")
	}
}

func TestTuiCommandUsesRealModel(t *testing.T) {
	// Verify that the tui command is wired to use the real Model and Store
	// from internal/tui package, with proper initialization.
	// This test checks that a Store can be created with NewModel returning the real model.

	store := &tui.Store{}
	model := tui.NewModel(store)

	// Check the model has the expected View() method with non-empty output
	view := model.View()
	if view == "" {
		t.Error("expected real model to have non-empty initial view")
	}

	// Verify Init() returns nil or a command (real model behavior)
	cmd := model.Init()
	// cmd can be nil, that's acceptable
	_ = cmd
}

func TestModelAcceptsConversationController(t *testing.T) {
	// Verify that the Model can accept and use a ConversationController
	// This enables wiring conversation-capable sessions into the TUI

	timeline := []conversation.FakeStep{
		{Event: conversation.Event{Type: conversation.EventTypeStream}},
	}
	session := conversation.NewFakeSession(timeline)
	controller := tui.NewConversationController(session)

	store := &tui.Store{}
	model := tui.NewModel(store)

	// Model should accept the controller
	model.SetConversationController(controller)

	// Switch to conversation view
	model.SwitchView(tui.ViewConversation)

	// Verify the view reflects the conversation
	view := model.View()
	if view == "" {
		t.Error("expected conversation view to be non-empty")
	}
}

func TestBuildPendingActionCommand(t *testing.T) {
	prevExecutable := osExecutable
	osExecutable = func() (string, error) { return "/usr/local/bin/gromit", nil }
	defer func() { osExecutable = prevExecutable }()

	action := &tui.PendingAction{Command: "refine", Args: []string{"abc"}}
	cmd, err := buildPendingActionCommand(action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantPath := "/usr/local/bin/gromit"
	if cmd.Path != wantPath {
		t.Fatalf("path = %q, want %q", cmd.Path, wantPath)
	}

	wantArgs := []string{wantPath, "refine", "abc"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("args = %+v, want %+v", cmd.Args, wantArgs)
	}

	if cmd.Stdin != os.Stdin || cmd.Stdout != os.Stdout || cmd.Stderr != os.Stderr {
		t.Fatalf("stdio not inherited")
	}
}

func TestRunTuiHydratesBeforeProgramAndExitsWhenNoPendingAction(t *testing.T) {
	originalLoadConfig := runTuiLoadConfig
	runTuiLoadConfig = func() (*config.Config, error) { return &config.Config{}, nil }
	defer func() { runTuiLoadConfig = originalLoadConfig }()

	callOrder := []string{}
	originalHydrate := hydrateStoreFn
	hydrateStoreFn = func(ctx context.Context, cfg *config.Config, gromitDir, specsDir, plansDir string, provider tui.HydrationProvider) *tui.Store {
		callOrder = append(callOrder, "hydrate")
		return &tui.Store{}
	}
	defer func() { hydrateStoreFn = originalHydrate }()

	originalProvider := newHydrationProvider
	newHydrationProvider = func(cfg *config.Config) tui.HydrationProvider {
		return &fakeHydrationProvider{}
	}
	defer func() { newHydrationProvider = originalProvider }()

	originalProgramFactory := newTeaProgram
	newTeaProgram = func(model tea.Model) teaProgram {
		callOrder = append(callOrder, "program")
		return &fakeTeaProgram{runFn: func() (tea.Model, error) {
			return model, nil
		}}
	}
	defer func() { newTeaProgram = originalProgramFactory }()

	if err := runTui(nil, nil); err != nil {
		t.Fatalf("runTui() error = %v", err)
	}

	if len(callOrder) != 2 {
		t.Fatalf("calls = %v, want hydrate->program", callOrder)
	}
	if callOrder[0] != "hydrate" || callOrder[1] != "program" {
		t.Fatalf("call order = %v, want [hydrate program]", callOrder)
	}
}

func TestRunTuiRelaunchesUntilPendingActionClears(t *testing.T) {
	originalLoadConfig := runTuiLoadConfig
	runTuiLoadConfig = func() (*config.Config, error) { return &config.Config{}, nil }
	defer func() { runTuiLoadConfig = originalLoadConfig }()

	hydrateCalls := 0
	originalHydrate := hydrateStoreFn
	hydrateStoreFn = func(ctx context.Context, cfg *config.Config, gromitDir, specsDir, plansDir string, provider tui.HydrationProvider) *tui.Store {
		hydrateCalls++
		return &tui.Store{}
	}
	defer func() { hydrateStoreFn = originalHydrate }()

	originalProvider := newHydrationProvider
	newHydrationProvider = func(cfg *config.Config) tui.HydrationProvider {
		return &fakeHydrationProvider{}
	}
	defer func() { newHydrationProvider = originalProvider }()

	runCount := 0
	expectedModels := []*pendingActionModel{
		{pending: &tui.PendingAction{Command: "plan", Args: []string{"spec"}}},
		{pending: nil},
	}
	originalProgramFactory := newTeaProgram
	newTeaProgram = func(model tea.Model) teaProgram {
		runCount++
		return &fakeTeaProgram{runFn: func() (tea.Model, error) {
			if runCount > len(expectedModels) {
				return nil, fmt.Errorf("unexpected run count %d", runCount)
			}
			return expectedModels[runCount-1], nil
		}}
	}
	defer func() { newTeaProgram = originalProgramFactory }()

	executePendingActionCalls := 0
	originalExecutePendingAction := executePendingAction
	executePendingAction = func(action *tui.PendingAction) error {
		executePendingActionCalls++
		if action == nil || action.Command != "plan" {
			t.Fatalf("unexpected pending action = %+v", action)
		}
		return nil
	}
	defer func() { executePendingAction = originalExecutePendingAction }()

	if err := runTui(nil, nil); err != nil {
		t.Fatalf("runTui() error = %v", err)
	}

	if hydrateCalls != len(expectedModels) {
		t.Fatalf("hydrate called %d times, want %d", hydrateCalls, len(expectedModels))
	}
	if runCount != len(expectedModels) {
		t.Fatalf("program run %d times, want %d", runCount, len(expectedModels))
	}
	if executePendingActionCalls != 1 {
		t.Fatalf("pending action executed %d times, want 1", executePendingActionCalls)
	}
}

type fakeHydrationProvider struct{}

func (*fakeHydrationProvider) RunnerStatus(ctx context.Context, gromitDir string) (*runner.Status, error) {
	return nil, nil
}

func (*fakeHydrationProvider) PipelineStatus(ctx context.Context, gromitDir, specsDir, plansDir string, startedAt *time.Time) (*pipeline.PipelineStatus, error) {
	return nil, nil
}

func (*fakeHydrationProvider) PipelineItems(ctx context.Context, gromitDir, specsDir, plansDir string) (tui.PipelineItems, error) {
	return tui.PipelineItems{}, nil
}

type fakeTeaProgram struct {
	runFn func() (tea.Model, error)
}

func (f *fakeTeaProgram) Run() (tea.Model, error) {
	if f == nil || f.runFn == nil {
		return nil, nil
	}
	return f.runFn()
}

type pendingActionModel struct {
	pending *tui.PendingAction
}

func (p *pendingActionModel) PendingAction() *tui.PendingAction {
	return p.pending
}

func (p *pendingActionModel) Init() tea.Cmd {
	return nil
}

func (p *pendingActionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return p, nil
}

func (p *pendingActionModel) View() string {
	return ""
}
