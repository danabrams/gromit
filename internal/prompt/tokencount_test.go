package prompt

import (
	"reflect"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "empty string", text: "", want: 0},
		{name: "single character", text: "a", want: 1},
		{name: "exact multiple of four", text: "abcd", want: 1},
		{name: "round up", text: "abcde", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokens(tt.text)
			if got != tt.want {
				t.Fatalf("EstimateTokens(%q) = %d, want %d", tt.text, got, tt.want)
			}
		})
	}
}

func TestEstimateTokensDeterministic(t *testing.T) {
	text := "consistent input for deterministic token estimation"

	first := EstimateTokens(text)
	for i := 0; i < 10; i++ {
		if got := EstimateTokens(text); got != first {
			t.Fatalf("EstimateTokens changed across calls: first=%d got=%d", first, got)
		}
	}
}

func TestEstimateSectionTokens(t *testing.T) {
	sections := map[string]string{
		"Rules": "rule text",
		"Spec":  "specification",
		"Task":  "",
	}

	got := EstimateSectionTokens(sections)

	if len(got) != len(sections) {
		t.Fatalf("EstimateSectionTokens returned %d keys, want %d", len(got), len(sections))
	}

	for key, text := range sections {
		value, ok := got[key]
		if !ok {
			t.Fatalf("EstimateSectionTokens missing key %q", key)
		}
		want := EstimateTokens(text)
		if value != want {
			t.Fatalf("EstimateSectionTokens[%q] = %d, want %d", key, value, want)
		}
	}
}

func TestEstimateSectionTokensNilInput(t *testing.T) {
	got := EstimateSectionTokens(nil)
	if !reflect.DeepEqual(got, map[string]int{}) {
		t.Fatalf("EstimateSectionTokens(nil) = %#v, want empty map", got)
	}
}
