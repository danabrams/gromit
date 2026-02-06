package retro

import (
	"context"
	"testing"
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
