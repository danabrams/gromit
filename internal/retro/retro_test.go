package retro

import (
	"context"
	"testing"

	"github.com/danabrams/ralph-runner/internal/claude"
)

func TestNewRetroNilConfig(t *testing.T) {
	r := NewRetro(nil, ".ralph")
	if r != nil {
		t.Error("expected nil Retro for nil config")
	}
}

func TestRunNilReceiver(t *testing.T) {
	var r *Retro
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil retro")
	}
}

func TestRunNilClaudeClient(t *testing.T) {
	r := &Retro{
		claude: nil,
	}
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil claude client")
	}
}

func TestRunNilLearningsFile(t *testing.T) {
	r := &Retro{
		claude:        claude.NewClient("claude", nil, 60),
		learningsFile: nil,
	}
	_, err := r.Run(context.Background(), false)
	if err == nil {
		t.Error("expected error for nil learnings file")
	}
}
