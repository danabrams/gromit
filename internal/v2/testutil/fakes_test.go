package testutil

import (
	"github.com/danabrams/gromit/internal/v2/adapter"
	"github.com/danabrams/gromit/internal/v2/adapter/llm"
	"github.com/danabrams/gromit/internal/v2/adapter/tasktracker"
)

var (
	_ llm.LLMProvider            = (*FakeLLM)(nil)
	_ tasktracker.TaskTracker    = (*FakeTaskTracker)(nil)
	_ adapter.TaskTrackerAdapter = (*FakeTaskTracker)(nil)
	_ adapter.PresenterAdapter   = (*FakePresenter)(nil)
	_ adapter.GitAdapter         = (*FakeGit)(nil)
)
