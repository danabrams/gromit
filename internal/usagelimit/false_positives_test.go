package usagelimit

import (
	"testing"
)

// TestClaudePatterns_NarrowKeywords verifies ClaudePatterns() returns only specific usage-limit keywords,
// not broad keywords that cause false positives.
// Expected failure: ClaudePatterns() currently includes 'exceeded', 'capacity', 'overloaded' keywords
func TestClaudePatterns_NarrowKeywords(t *testing.T) {
	patterns := ClaudePatterns()

	// Verify specific keywords are present
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

	// Verify broad keywords are NOT present
	broadKeywords := []string{"exceeded", "capacity", "overloaded"}
	for _, broad := range broadKeywords {
		for _, keyword := range patterns.Keywords {
			if keyword == broad {
				t.Errorf("ClaudePatterns() contains overly broad keyword %q that causes false positives", broad)
			}
		}
	}
}

// TestCodexPatterns_NarrowKeywords verifies CodexPatterns() returns only specific usage-limit keywords.
// Expected failure: CodexPatterns() currently includes 'exceeded' keyword
func TestCodexPatterns_NarrowKeywords(t *testing.T) {
	patterns := CodexPatterns()

	// Verify specific keywords are present
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

	// Verify broad keyword is NOT present
	for _, keyword := range patterns.Keywords {
		if keyword == "exceeded" {
			t.Errorf("CodexPatterns() contains overly broad keyword 'exceeded' that causes false positives")
		}
	}
}

// TestCheck_LegitimateErrorsNotDetectedAsUsageLimits verifies that legitimate error messages
// containing broad keywords like 'exceeded', 'capacity', 'overloaded' are NOT detected as usage limits
// when using the narrowed pattern list.
// Expected failure: Check() will return true for these cases with current broad keywords in patterns
func TestCheck_LegitimateErrorsNotDetectedAsUsageLimits(t *testing.T) {
	// Use the actual patterns that will be returned after the fix
	patterns := Patterns{
		Keywords: []string{"usage limit", "rate limit", "quota", "too many requests", "429"},
	}

	tests := []struct {
		name     string
		signals  Signals
		wantFail bool // true if we want Check to return false (not a usage limit)
	}{
		{
			name: "buffer capacity exceeded is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: buffer capacity exceeded - resize buffer",
			},
			wantFail: true,
		},
		{
			name: "array index exceeded bounds is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "panic: runtime error: index exceeded array bounds",
			},
			wantFail: true,
		},
		{
			name: "max retry count exceeded is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: max retry count exceeded (3 attempts)",
			},
			wantFail: true,
		},
		{
			name: "capacity planning error is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: capacity planning failed - insufficient resources",
			},
			wantFail: true,
		},
		{
			name: "system overloaded with connections is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Warning: system overloaded with 1000+ concurrent connections",
			},
			wantFail: true,
		},
		{
			name: "disk capacity issue is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: disk capacity at 95% - cleanup required",
			},
			wantFail: true,
		},
		{
			name: "server overloaded generic error is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: server overloaded - try again later",
			},
			wantFail: true,
		},
		{
			name: "memory exceeded limit is not a usage limit",
			signals: Signals{
				ExitCode: 137,
				Output:   "Fatal: memory exceeded limit - process killed",
			},
			wantFail: true,
		},
		{
			name: "timeout exceeded is not a usage limit",
			signals: Signals{
				ExitCode: 124,
				Output:   "Error: timeout exceeded after 30 seconds",
			},
			wantFail: true,
		},
		{
			name: "CPU capacity warning is not a usage limit",
			signals: Signals{
				ExitCode: 1,
				Output:   "Warning: CPU capacity at maximum - throttling enabled",
			},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, patterns)
			if tt.wantFail && got {
				t.Errorf("Check() = true (detected as usage limit), want false - %q should not trigger usage limit detection", tt.signals.Output)
			}
		})
	}
}

// TestCheck_SpecificPatternsStillWork verifies that after removing broad keywords,
// specific usage-limit messages are still correctly detected.
// Expected failure: Tests will pass initially but exist to document expected behavior
func TestCheck_SpecificPatternsStillWork(t *testing.T) {
	// Use the actual patterns that will be returned after the fix
	patterns := Patterns{
		Keywords: []string{"usage limit", "rate limit", "quota", "too many requests", "429"},
	}

	tests := []struct {
		name    string
		signals Signals
		want    bool
	}{
		{
			name: "usage limit detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: usage limit exceeded for your plan",
			},
			want: true,
		},
		{
			name: "rate limit detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: rate limit reached - please wait",
			},
			want: true,
		},
		{
			name: "quota detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: quota exceeded for this billing period",
			},
			want: true,
		},
		{
			name: "too many requests detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: too many requests - backoff required",
			},
			want: true,
		},
		{
			name: "429 status code detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "HTTP 429 Too Many Requests",
			},
			want: true,
		},
		{
			name: "rate limit with exceeded should still match via rate limit keyword",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: rate limit exceeded",
			},
			want: true,
		},
		{
			name: "quota with exceeded should still match via quota keyword",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: quota exceeded",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v - specific pattern %q should be detected", got, tt.want, tt.signals.Output)
			}
		})
	}
}

// TestCheck_RateLimitHitsPathUnaffected verifies that the RateLimitHits signal path
// continues to work correctly and handles cases where keywords don't match.
// Expected failure: No expected failure - validates that existing behavior is preserved
func TestCheck_RateLimitHitsPathUnaffected(t *testing.T) {
	patterns := Patterns{
		Keywords: []string{"usage limit", "rate limit", "quota", "too many requests", "429"},
	}

	tests := []struct {
		name    string
		signals Signals
		want    bool
	}{
		{
			name: "rate limit hits with generic error message detected as usage limit",
			signals: Signals{
				ExitCode:      1,
				Output:        "Error: operation failed",
				RateLimitHits: 3,
			},
			want: true,
		},
		{
			name: "rate limit hits with buffer capacity error detected as usage limit",
			signals: Signals{
				ExitCode:      1,
				Output:        "Error: buffer capacity exceeded",
				RateLimitHits: 1,
			},
			want: true,
		},
		{
			name: "rate limit hits takes precedence over absence of keywords",
			signals: Signals{
				ExitCode:      1,
				Output:        "unexpected error occurred",
				RateLimitHits: 2,
			},
			want: true,
		},
		{
			name: "zero rate limit hits with generic exceeded error not detected",
			signals: Signals{
				ExitCode:      1,
				Output:        "Error: timeout exceeded",
				RateLimitHits: 0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v - RateLimitHits path should work independently of keywords", got, tt.want)
			}
		})
	}
}

// TestCheck_EdgeCasesWithNarrowedKeywords verifies edge cases after keyword narrowing.
// Expected failure: Some tests may fail initially with current broad keywords
func TestCheck_EdgeCasesWithNarrowedKeywords(t *testing.T) {
	patterns := Patterns{
		Keywords: []string{"usage limit", "rate limit", "quota", "too many requests", "429"},
	}

	tests := []struct {
		name    string
		signals Signals
		want    bool
	}{
		{
			name: "exceeded alone without usage/rate/quota context not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: limit exceeded",
			},
			want: false,
		},
		{
			name: "capacity alone without usage context not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: capacity reached",
			},
			want: false,
		},
		{
			name: "overloaded alone without rate limit context not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: system overloaded",
			},
			want: false,
		},
		{
			name: "usage limit phrase detected regardless of exceeded presence",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: usage limit has been reached",
			},
			want: true,
		},
		{
			name: "rate limit phrase detected regardless of exceeded presence",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: rate limit has been reached",
			},
			want: true,
		},
		{
			name: "multiple broad keywords without specific context not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: capacity exceeded and system overloaded",
			},
			want: false,
		},
		{
			name: "too many requests detected even without exceeded keyword",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: too many requests from your IP",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(tt.signals, patterns)
			if got != tt.want {
				t.Errorf("Check() = %v, want %v for output: %q", got, tt.want, tt.signals.Output)
			}
		})
	}
}

// TestCheck_RealWorldFalsePositives verifies that real-world error scenarios
// that were previously triggering false positives are now correctly handled.
// Expected failure: These will fail with current broad keywords in ClaudePatterns/CodexPatterns
func TestCheck_RealWorldFalsePositives(t *testing.T) {
	claudePatterns := ClaudePatterns()
	codexPatterns := CodexPatterns()

	tests := []struct {
		name             string
		signals          Signals
		useClaudePattern bool
		want             bool
	}{
		{
			name: "Go panic with array bounds exceeded not detected",
			signals: Signals{
				ExitCode: 2,
				Output:   "panic: runtime error: index out of range [10] with length 5\ngoroutine 1:\nmain.go:42: index exceeded array bounds",
			},
			useClaudePattern: true,
			want:             false,
		},
		{
			name: "Docker container memory exceeded not detected",
			signals: Signals{
				ExitCode: 137,
				Output:   "Container killed: memory limit exceeded (OOMKilled)",
			},
			useClaudePattern: true,
			want:             false,
		},
		{
			name: "Database connection pool capacity not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "ERROR: connection pool capacity reached - max 100 connections",
			},
			useClaudePattern: true,
			want:             false,
		},
		{
			name: "Kubernetes pod overloaded not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Warning: pod overloaded - CPU throttling active",
			},
			useClaudePattern: true,
			want:             false,
		},
		{
			name: "Build timeout exceeded not detected",
			signals: Signals{
				ExitCode: 124,
				Output:   "Build failed: maximum build time exceeded (30 minutes)",
			},
			useClaudePattern: true,
			want:             false,
		},
		{
			name: "Codex buffer capacity not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: output buffer capacity exceeded - truncating",
			},
			useClaudePattern: false,
			want:             false,
		},
		{
			name: "Codex retry count exceeded not detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: maximum retry count exceeded",
			},
			useClaudePattern: false,
			want:             false,
		},
		{
			name: "Claude actual usage limit still detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: Claude API usage limit exceeded for your organization",
			},
			useClaudePattern: true,
			want:             true,
		},
		{
			name: "Claude actual rate limit still detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: rate limit exceeded - please wait 60 seconds",
			},
			useClaudePattern: true,
			want:             true,
		},
		{
			name: "Codex actual quota still detected",
			signals: Signals{
				ExitCode: 1,
				Output:   "Error: monthly quota exceeded - upgrade plan to continue",
			},
			useClaudePattern: false,
			want:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patterns Patterns
			if tt.useClaudePattern {
				patterns = claudePatterns
			} else {
				patterns = codexPatterns
			}

			got := Check(tt.signals, patterns)
			if got != tt.want {
				if tt.want {
					t.Errorf("Check() = false, want true - legitimate usage limit should be detected: %q", tt.signals.Output)
				} else {
					t.Errorf("Check() = true, want false - false positive should be prevented: %q", tt.signals.Output)
				}
			}
		})
	}
}
