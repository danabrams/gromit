package usagelimit

import (
	"testing"

	"github.com/danabrams/gromit/internal/claude"
	"github.com/danabrams/gromit/internal/logger"
)

func TestCheck_ExitCodeAndStderrPattern(t *testing.T) {
	tests := []struct {
		name     string
		signals  Signals
		patterns Patterns
		want     bool
	}{
		{
			name: "usage limit in stderr with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: usage limit exceeded for your plan",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota"},
			},
			want: true,
		},
		{
			name: "rate limit in stderr with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: rate limit reached",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota"},
			},
			want: true,
		},
		{
			name: "quota exceeded in stdout with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "quota exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota"},
			},
			want: true,
		},
		{
			name: "capacity error with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "insufficient capacity",
			},
			patterns: Patterns{
				Keywords: []string{"capacity", "overloaded"},
			},
			want: true,
		},
		{
			name: "overloaded error with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "system overloaded",
			},
			patterns: Patterns{
				Keywords: []string{"capacity", "overloaded"},
			},
			want: true,
		},
		{
			name: "too many requests with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "too many requests",
			},
			patterns: Patterns{
				Keywords: []string{"too many requests", "429"},
			},
			want: true,
		},
		{
			name: "429 status code with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "HTTP 429 response",
			},
			patterns: Patterns{
				Keywords: []string{"too many requests", "429"},
			},
			want: true,
		},
		{
			name: "exceeded keyword with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Request limit exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"exceeded"},
			},
			want: true,
		},
		{
			name: "usage limit keyword but exit code 0 (success)",
			signals: Signals{
				ExitCode: 0,
				Output:   "usage limit exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name: "non-zero exit but no matching keywords",
			signals: Signals{
				ExitCode: 1,
				Output:   "syntax error in code",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
		{
			name: "test failure output should not match",
			signals: Signals{
				ExitCode: 1,
				Output:   "FAIL: TestFoo (0.01s)\n--- FAIL: TestBar\nFAILED\n",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
		{
			name: "build error should not match",
			signals: Signals{
				ExitCode: 2,
				Output:   "build failed: undefined: foo",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
		{
			name: "empty output with non-zero exit",
			signals: Signals{
				ExitCode: 1,
				Output:   "",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_RateLimitHitsPath(t *testing.T) {
	tests := []struct {
		name     string
		signals  Signals
		patterns Patterns
		want     bool
	}{
		{
			name: "rate limit hits with failed invocation",
			signals: Signals{
				ExitCode:      1,
				Output:        "some output",
				RateLimitHits: 3,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name: "rate limit hits with successful invocation",
			signals: Signals{
				ExitCode:      0,
				Output:        "success",
				RateLimitHits: 2,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name: "zero rate limit hits with failed invocation",
			signals: Signals{
				ExitCode:      1,
				Output:        "error",
				RateLimitHits: 0,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name: "rate limit hits takes precedence over keyword absence",
			signals: Signals{
				ExitCode:      1,
				Output:        "generic error",
				RateLimitHits: 1,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_CaseInsensitivity(t *testing.T) {
	tests := []struct {
		name     string
		signals  Signals
		patterns Patterns
		want     bool
	}{
		{
			name: "uppercase keyword in output",
			signals: Signals{
				ExitCode: 1,
				Output:   "USAGE LIMIT EXCEEDED",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name: "mixed case keyword in output",
			signals: Signals{
				ExitCode: 1,
				Output:   "Rate Limit Reached",
			},
			patterns: Patterns{
				Keywords: []string{"rate limit"},
			},
			want: true,
		},
		{
			name: "lowercase keyword matches uppercase pattern",
			signals: Signals{
				ExitCode: 1,
				Output:   "quota exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"QUOTA"},
			},
			want: true,
		},
		{
			name: "case variations in 429",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error 429 Too Many Requests",
			},
			patterns: Patterns{
				Keywords: []string{"429", "too many requests"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		signals  Signals
		patterns Patterns
		want     bool
	}{
		{
			name: "empty patterns list",
			signals: Signals{
				ExitCode: 1,
				Output:   "usage limit exceeded",
			},
			patterns: Patterns{
				Keywords: []string{},
			},
			want: false,
		},
		{
			name: "nil patterns list",
			signals: Signals{
				ExitCode: 1,
				Output:   "usage limit exceeded",
			},
			patterns: Patterns{
				Keywords: nil,
			},
			want: false,
		},
		{
			name: "empty output and zero rate limit hits",
			signals: Signals{
				ExitCode:      1,
				Output:        "",
				RateLimitHits: 0,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name: "keyword appears in middle of sentence",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: your usage limit has been exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name: "multiple keywords, one matches",
			signals: Signals{
				ExitCode: 1,
				Output:   "rate limit exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota"},
			},
			want: true,
		},
		{
			name: "multiple keywords, none match",
			signals: Signals{
				ExitCode: 1,
				Output:   "compilation error",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota"},
			},
			want: false,
		},
		{
			name: "exit code 255 (common CLI error)",
			signals: Signals{
				ExitCode: 255,
				Output:   "rate limit exceeded",
			},
			patterns: Patterns{
				Keywords: []string{"rate limit"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_CombinedSignals(t *testing.T) {
	tests := []struct {
		name     string
		signals  Signals
		patterns Patterns
		want     bool
	}{
		{
			name: "both keyword match and rate limit hits",
			signals: Signals{
				ExitCode:      1,
				Output:        "usage limit exceeded",
				RateLimitHits: 2,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name: "rate limit hits but exit 0 and keyword absent",
			signals: Signals{
				ExitCode:      0,
				Output:        "success",
				RateLimitHits: 1,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name: "keyword match overrides zero rate limit hits",
			signals: Signals{
				ExitCode:      1,
				Output:        "quota exceeded",
				RateLimitHits: 0,
			},
			patterns: Patterns{
				Keywords: []string{"quota"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudePatterns(t *testing.T) {
	patterns := ClaudePatterns()

	if patterns.Keywords == nil {
		t.Fatal("ClaudePatterns() returned nil Keywords")
	}

	// Verify known Claude-specific patterns are present (narrowed to prevent false positives)
	expectedKeywords := []string{"usage limit", "rate limit", "quota", "too many requests", "429"}
	for _, expected := range expectedKeywords {
		found := false
		for _, keyword := range patterns.Keywords {
			if keyword == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ClaudePatterns() missing expected keyword: %q", expected)
		}
	}

	if len(patterns.Keywords) == 0 {
		t.Error("ClaudePatterns() returned empty Keywords list")
	}
}

func TestCodexPatterns(t *testing.T) {
	patterns := CodexPatterns()

	if patterns.Keywords == nil {
		t.Fatal("CodexPatterns() returned nil Keywords")
	}

	// Verify common limit patterns are present (narrowed to prevent false positives)
	expectedKeywords := []string{"usage limit", "rate limit", "quota"}
	for _, expected := range expectedKeywords {
		found := false
		for _, keyword := range patterns.Keywords {
			if keyword == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CodexPatterns() missing expected keyword: %q", expected)
		}
	}

	if len(patterns.Keywords) == 0 {
		t.Error("CodexPatterns() returned empty Keywords list")
	}
}

func TestCheck_WithClaudeResult(t *testing.T) {
	// Tests integration with the actual claude.Result type
	tests := []struct {
		name          string
		result        *claude.Result
		rateLimitHits int
		patterns      Patterns
		want          bool
	}{
		{
			name: "failed result with usage limit in output",
			result: &claude.Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Error: usage limit exceeded",
			},
			rateLimitHits: 0,
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name: "failed result with rate limit hits",
			result: &claude.Result{
				Success:  false,
				ExitCode: 1,
				Output:   "Some error occurred",
			},
			rateLimitHits: 3,
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name: "successful result should not trigger detection",
			result: &claude.Result{
				Success:  true,
				ExitCode: 0,
				Output:   "Task completed",
			},
			rateLimitHits: 0,
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name:          "nil result",
			result:        nil,
			rateLimitHits: 0,
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := Signals{}
			if tt.result != nil {
				signals.ExitCode = tt.result.ExitCode
				signals.Output = tt.result.Output
			}
			signals.RateLimitHits = tt.rateLimitHits

			got := Check(signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_WithStreamStats(t *testing.T) {
	// Tests integration with the actual logger.StreamStats type
	tests := []struct {
		name     string
		exitCode int
		output   string
		stats    *logger.StreamStats
		patterns Patterns
		want     bool
	}{
		{
			name:     "stream stats with rate limit hits and failure",
			exitCode: 1,
			output:   "invocation failed",
			stats: func() *logger.StreamStats {
				s, _ := logger.NewStreamStats()
				s.RateLimitHits = 2
				return s
			}(),
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: true,
		},
		{
			name:     "stream stats with zero rate limit hits",
			exitCode: 1,
			output:   "invocation failed",
			stats: func() *logger.StreamStats {
				s, _ := logger.NewStreamStats()
				return s
			}(),
			patterns: Patterns{
				Keywords: []string{"usage limit"},
			},
			want: false,
		},
		{
			name:     "nil stream stats",
			exitCode: 1,
			output:   "rate limit exceeded",
			stats:    nil,
			patterns: Patterns{
				Keywords: []string{"rate limit"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := Signals{
				ExitCode: tt.exitCode,
				Output:   tt.output,
			}
			if tt.stats != nil {
				signals.RateLimitHits = tt.stats.RateLimitHits
			}

			got := Check(signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheck_FalsePositivePrevention(t *testing.T) {
	// Ensures normal code/test/build failures are NOT detected as usage limits
	tests := []struct {
		name     string
		signals  Signals
		patterns Patterns
		want     bool
	}{
		{
			name: "go test failure output",
			signals: Signals{
				ExitCode: 1,
				Output: `=== RUN   TestFoo
--- FAIL: TestFoo (0.01s)
    foo_test.go:10: expected true, got false
FAIL
FAIL	github.com/example/pkg	0.023s`,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota", "exceeded", "capacity", "overloaded"},
			},
			want: false,
		},
		{
			name: "go build error",
			signals: Signals{
				ExitCode: 2,
				Output:   "# github.com/example/pkg\n./main.go:10:2: undefined: foo",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit", "quota"},
			},
			want: false,
		},
		{
			name: "golangci-lint failure",
			signals: Signals{
				ExitCode: 1,
				Output:   "main.go:10:1: exported function Foo should have comment or be unexported (revive)",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
		{
			name: "npm test failure",
			signals: Signals{
				ExitCode: 1,
				Output: `FAIL src/components/Button.test.tsx
  ● Button › renders correctly
    expect(received).toBe(expected)`,
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
		{
			name: "generic command failure",
			signals: Signals{
				ExitCode: 1,
				Output:   "command not found: foobar",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
		{
			name: "timeout error (not a usage limit)",
			signals: Signals{
				ExitCode: 124,
				Output:   "command timed out after 30 seconds",
			},
			patterns: Patterns{
				Keywords: []string{"usage limit", "rate limit"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, tt.patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v (false positive detected)", got, tt.want)
			}
		})
	}
}

func TestClaudePatterns_NoFalsePositiveKeywords(t *testing.T) {
	// Verify that overly broad keywords that cause false positives are not in ClaudePatterns
	patterns := ClaudePatterns()

	// These keywords should NOT be present as they match legitimate errors
	forbiddenKeywords := []string{"exceeded", "capacity", "overloaded"}
	for _, forbidden := range forbiddenKeywords {
		for _, keyword := range patterns.Keywords {
			if keyword == forbidden {
				t.Errorf("ClaudePatterns() contains broad keyword %q which causes false positives", forbidden)
			}
		}
	}

	// These keywords SHOULD be present as they are specific to usage limits
	requiredKeywords := []string{"usage limit", "rate limit", "quota", "too many requests", "429"}
	for _, required := range requiredKeywords {
		found := false
		for _, keyword := range patterns.Keywords {
			if keyword == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ClaudePatterns() missing required specific keyword: %q", required)
		}
	}
}

func TestCodexPatterns_NoFalsePositiveKeywords(t *testing.T) {
	// Verify that overly broad keywords that cause false positives are not in CodexPatterns
	patterns := CodexPatterns()

	// "exceeded" should NOT be present as it matches legitimate errors
	forbiddenKeywords := []string{"exceeded"}
	for _, forbidden := range forbiddenKeywords {
		for _, keyword := range patterns.Keywords {
			if keyword == forbidden {
				t.Errorf("CodexPatterns() contains broad keyword %q which causes false positives", forbidden)
			}
		}
	}

	// These keywords SHOULD be present as they are specific to usage limits
	requiredKeywords := []string{"usage limit", "rate limit", "quota"}
	for _, required := range requiredKeywords {
		found := false
		for _, keyword := range patterns.Keywords {
			if keyword == required {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CodexPatterns() missing required specific keyword: %q", required)
		}
	}
}

func TestSignals_Struct(t *testing.T) {
	// Verifies Signals struct has the expected fields
	s := Signals{
		ExitCode:      1,
		Output:        "test output",
		RateLimitHits: 3,
	}

	if s.ExitCode != 1 {
		t.Errorf("Signals.ExitCode = %d, want 1", s.ExitCode)
	}
	if s.Output != "test output" {
		t.Errorf("Signals.Output = %q, want %q", s.Output, "test output")
	}
	if s.RateLimitHits != 3 {
		t.Errorf("Signals.RateLimitHits = %d, want 3", s.RateLimitHits)
	}
}

func TestPatterns_Struct(t *testing.T) {
	// Verifies Patterns struct has the expected fields
	p := Patterns{
		Keywords: []string{"usage limit", "rate limit"},
	}

	if len(p.Keywords) != 2 {
		t.Errorf("Patterns.Keywords length = %d, want 2", len(p.Keywords))
	}
	if p.Keywords[0] != "usage limit" {
		t.Errorf("Patterns.Keywords[0] = %q, want %q", p.Keywords[0], "usage limit")
	}
	if p.Keywords[1] != "rate limit" {
		t.Errorf("Patterns.Keywords[1] = %q, want %q", p.Keywords[1], "rate limit")
	}
}
