package event

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"
)

func TestEventDefinitions(t *testing.T) {
	t.Run("BaseEventFields", func(t *testing.T) {
		t.Helper()
		typ := reflect.TypeOf(Event{})
		for _, field := range []string{"SchemaVersion", "Timestamp", "Type"} {
			if _, ok := typ.FieldByName(field); !ok {
				t.Fatalf("expected Event to expose %s", field)
			}
		}
		if SchemaVersion == 0 {
			t.Fatalf("schema version must be non-zero")
		}
	})

	t.Run("LifecycleEventsEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(SpecStartedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(SpecCompletedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(SpecFailedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(BeadStartedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(BeadCompletedEvent{}))
	})

	t.Run("StageEventsEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(StageStartedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(StageCompletedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(StageFailedEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(StageRetryingEvent{}))
	})

	t.Run("ValidationReviewScopeTelemetryEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(ValidationEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(ReviewEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(ScopeEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(TelemetryEvent{}))
	})

	t.Run("BuildInvocationAndModelEventsEmbedBase", func(t *testing.T) {
		expectEmbeddedEvent(t, reflect.TypeOf(BuildInvocationStartEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(BuildInvocationCompleteEvent{}))
		expectEmbeddedEvent(t, reflect.TypeOf(ModelSelectedEvent{}))
	})
}

func TestEventJSONRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		event          interface{}
		expectedFields []string
	}{
		{
			name: "SpecStartedEvent",
			event: SpecStartedEvent{
				Event:    Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeSpecStarted},
				SpecID:   "immutable-pipeline",
				Worktree: "/tmp/gromit-worktree-abc12",
			},
			expectedFields: []string{"schema_version", "timestamp", "type", "spec_id", "worktree"},
		},
		{
			name: "SpecCompletedEvent",
			event: SpecCompletedEvent{
				Event:         Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeSpecCompleted},
				SpecID:        "immutable-pipeline",
				Worktree:      "/tmp/gromit-worktree-abc12",
				Success:       true,
				FailureReason: "",
			},
			expectedFields: []string{"schema_version", "spec_id", "success"},
		},
		{
			name: "SpecFailedEvent",
			event: SpecFailedEvent{
				Event:         Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeSpecFailed},
				SpecID:        "immutable-pipeline",
				Worktree:      "/tmp/gromit-worktree-abc12",
				FailureReason: "build timeout exceeded",
			},
			expectedFields: []string{"schema_version", "spec_id", "failure_reason"},
		},
		{
			name: "BeadStartedEvent",
			event: BeadStartedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBeadStarted},
				BeadID:    "gromit-abc12",
				BeadTitle: "add validation retry logic",
				Iteration: 2,
			},
			expectedFields: []string{"schema_version", "bead_id", "bead_title", "iteration"},
		},
		{
			name: "BeadCompletedEvent",
			event: BeadCompletedEvent{
				Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBeadCompleted},
				BeadID:       "gromit-abc12",
				BeadTitle:    "add validation retry logic",
				Iteration:    2,
				Success:      true,
				RetryAttempt: 1,
				Model:        "claude-sonnet-4-20250514",
				Provider:     "anthropic",
				CostUSD:      0.042,
				InputTokens:  15000,
				OutputTokens: 3200,
				Duration:     2 * time.Minute,
			},
			expectedFields: []string{"schema_version", "bead_id", "bead_title", "iteration", "success", "retry_attempt", "model", "provider", "cost_usd", "input_tokens", "output_tokens", "duration"},
		},
		{
			name: "StageStartedEvent",
			event: StageStartedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageStarted},
				StageName: "build",
				BeadID:    "gromit-abc12",
				Iteration: 2,
			},
			expectedFields: []string{"schema_version", "stage_name", "bead_id", "iteration"},
		},
		{
			name: "StageCompletedEvent",
			event: StageCompletedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageCompleted},
				StageName: "build",
				BeadID:    "gromit-abc12",
				Iteration: 2,
				Success:   true,
				Duration:  3 * time.Minute,
			},
			expectedFields: []string{"schema_version", "stage_name", "bead_id", "success", "duration"},
		},
		{
			name: "StageFailedEvent",
			event: StageFailedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageFailed},
				StageName: "build",
				BeadID:    "gromit-abc12",
				Iteration: 2,
				Error:     "exit status 1: compilation failed",
			},
			expectedFields: []string{"schema_version", "stage_name", "bead_id", "error"},
		},
		{
			name: "StageRetryingEvent",
			event: StageRetryingEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageRetrying},
				StageName: "build",
				BeadID:    "gromit-abc12",
				Attempt:   3,
				Reason:    "escalating model after haiku failure",
			},
			expectedFields: []string{"schema_version", "stage_name", "bead_id", "attempt", "reason"},
		},
		{
			name: "ValidationEvent",
			event: ValidationEvent{
				Event:         Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeValidation},
				BeadID:        "gromit-abc12",
				StageName:     "validate",
				Commands:      []string{"go test ./...", "golangci-lint run"},
				FailedCommand: "go test ./...",
				Succeeded:     false,
				Duration:      45 * time.Second,
				Details:       "3 tests failed in internal/runner",
			},
			expectedFields: []string{"schema_version", "bead_id", "stage_name", "commands", "failed_command", "succeeded", "duration", "details"},
		},
		{
			name: "ReviewEvent",
			event: ReviewEvent{
				Event:      Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeReview},
				BeadID:     "gromit-abc12",
				Verdict:    "approve",
				Issues:     []string{"minor: missing doc comment on exported func"},
				OutOfScope: []string{"refactor logger package"},
				Notes:      "clean implementation overall",
			},
			expectedFields: []string{"schema_version", "bead_id", "verdict", "issues", "out_of_scope", "notes"},
		},
		{
			name: "ScopeEvent",
			event: ScopeEvent{
				Event:      Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeScope},
				BeadID:     "gromit-abc12",
				Complexity: "P1",
				Approved:   true,
				Reason:     "single-package change with clear spec",
			},
			expectedFields: []string{"schema_version", "bead_id", "complexity", "approved", "reason"},
		},
		{
			name: "TelemetryEvent",
			event: TelemetryEvent{
				Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeTelemetry},
				BeadID:       "gromit-abc12",
				StageName:    "build",
				Model:        "claude-sonnet-4-20250514",
				Duration:     2 * time.Minute,
				InputTokens:  15000,
				OutputTokens: 3200,
				CostUSD:      0.042,
				Category:     "generation",
			},
			expectedFields: []string{"schema_version", "bead_id", "stage_name", "model", "duration", "input_tokens", "output_tokens", "cost_usd", "category"},
		},
		{
			name: "GenerationCapReachedEvent",
			event: GenerationCapReachedEvent{
				Event:             Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeGenerationCapReached},
				GenerationCap:     5,
				HighestGeneration: 5,
			},
			expectedFields: []string{"schema_version", "generation_cap", "highest_generation"},
		},
		{
			name: "TriageStartedEvent",
			event: TriageStartedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeTriageStarted},
				BeadID:    "gromit-abc12",
				BeadTitle: "fix flaky test in runner",
				Iteration: 1,
			},
			expectedFields: []string{"schema_version", "bead_id", "bead_title", "iteration"},
		},
		{
			name: "TriageCompletedEvent",
			event: TriageCompletedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeTriageCompleted},
				BeadID:    "gromit-abc12",
				BeadTitle: "fix flaky test in runner",
				Iteration: 1,
				Category:  "bug-fix",
				Reasoning: "test relies on wall-clock timing",
			},
			expectedFields: []string{"schema_version", "bead_id", "bead_title", "category", "reasoning"},
		},
		{
			name: "BuildInvocationStartEvent",
			event: BuildInvocationStartEvent{
				Event:       Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBuildInvocationStart},
				BeadID:      "gromit-abc12",
				Model:       "claude-sonnet-4-20250514",
				Provider:    "anthropic",
				Tier:        "P1",
				Attempt:     1,
				MaxAttempts: 3,
			},
			expectedFields: []string{"schema_version", "bead_id", "model", "provider", "tier", "attempt", "max_attempts"},
		},
		{
			name: "BuildInvocationCompleteEvent",
			event: BuildInvocationCompleteEvent{
				Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBuildInvocationComplete},
				BeadID:       "gromit-abc12",
				Model:        "claude-sonnet-4-20250514",
				Provider:     "anthropic",
				Success:      true,
				Duration:     2 * time.Minute,
				CostUSD:      0.042,
				InputTokens:  15000,
				OutputTokens: 3200,
				PromptSize:   48000,
			},
			expectedFields: []string{"schema_version", "bead_id", "model", "provider", "success", "duration", "cost_usd", "input_tokens", "output_tokens", "prompt_size"},
		},
		{
			name: "ModelSelectedEvent",
			event: ModelSelectedEvent{
				Event:    Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeModelSelected},
				BeadID:   "gromit-abc12",
				Model:    "claude-sonnet-4-20250514",
				Provider: "anthropic",
				Tier:     "P1",
				Reason:   "complexity-based selection",
			},
			expectedFields: []string{"schema_version", "bead_id", "model", "provider", "tier", "reason"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.event)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			jsonStr := string(data)
			for _, field := range tc.expectedFields {
				if !strings.Contains(jsonStr, `"`+field+`"`) {
					t.Errorf("JSON output missing expected field %q in: %s", field, jsonStr)
				}
			}

			// Unmarshal back into the same concrete type and compare.
			dst := reflect.New(reflect.TypeOf(tc.event)).Interface()
			if err := json.Unmarshal(data, dst); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			got := reflect.ValueOf(dst).Elem().Interface()
			if !reflect.DeepEqual(tc.event, got) {
				t.Errorf("round-trip mismatch\n  original:     %+v\n  unmarshaled:  %+v", tc.event, got)
			}
		})
	}
}

func TestEventJSONFieldNames(t *testing.T) {
	now := time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC)

	// Build a set of events with all fields populated so omitempty does not hide them.
	events := []struct {
		name              string
		event             interface{}
		expectedSnakeCase []string
	}{
		{
			name: "SpecStartedEvent",
			event: SpecStartedEvent{
				Event:    Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeSpecStarted},
				SpecID:   "s1",
				Worktree: "/w",
			},
			expectedSnakeCase: []string{"schema_version", "spec_id"},
		},
		{
			name: "BeadCompletedEvent",
			event: BeadCompletedEvent{
				Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBeadCompleted},
				BeadID:       "b1",
				BeadTitle:    "t",
				Iteration:    1,
				Success:      true,
				RetryAttempt: 1,
				Model:        "m",
				Provider:     "p",
				CostUSD:      0.01,
				InputTokens:  100,
				OutputTokens: 50,
				Duration:     time.Second,
			},
			expectedSnakeCase: []string{"bead_id", "bead_title", "retry_attempt", "cost_usd", "input_tokens", "output_tokens"},
		},
		{
			name: "StageCompletedEvent",
			event: StageCompletedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageCompleted},
				StageName: "build",
				BeadID:    "b1",
				Iteration: 1,
				Success:   true,
				Duration:  time.Second,
			},
			expectedSnakeCase: []string{"stage_name", "bead_id", "schema_version"},
		},
		{
			name: "ValidationEvent",
			event: ValidationEvent{
				Event:         Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeValidation},
				BeadID:        "b1",
				StageName:     "v",
				Commands:      []string{"go test"},
				FailedCommand: "go test",
				Succeeded:     false,
				Duration:      time.Second,
				Details:       "d",
			},
			expectedSnakeCase: []string{"bead_id", "stage_name", "failed_command"},
		},
		{
			name: "TelemetryEvent",
			event: TelemetryEvent{
				Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeTelemetry},
				BeadID:       "b1",
				StageName:    "s",
				Model:        "m",
				Duration:     time.Second,
				InputTokens:  100,
				OutputTokens: 50,
				CostUSD:      0.01,
				Category:     "c",
			},
			expectedSnakeCase: []string{"input_tokens", "output_tokens", "cost_usd", "stage_name"},
		},
		{
			name: "ReviewEvent",
			event: ReviewEvent{
				Event:      Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeReview},
				BeadID:     "b1",
				Verdict:    "approve",
				Issues:     []string{"i"},
				OutOfScope: []string{"o"},
				Notes:      "n",
			},
			expectedSnakeCase: []string{"out_of_scope", "bead_id"},
		},
		{
			name: "GenerationCapReachedEvent",
			event: GenerationCapReachedEvent{
				Event:             Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeGenerationCapReached},
				GenerationCap:     5,
				HighestGeneration: 5,
			},
			expectedSnakeCase: []string{"generation_cap", "highest_generation"},
		},
		{
			name: "SpecCompletedEvent",
			event: SpecCompletedEvent{
				Event:         Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeSpecCompleted},
				SpecID:        "s1",
				Worktree:      "/w",
				Success:       true,
				FailureReason: "r",
			},
			expectedSnakeCase: []string{"schema_version", "spec_id", "failure_reason"},
		},
		{
			name: "SpecFailedEvent",
			event: SpecFailedEvent{
				Event:         Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeSpecFailed},
				SpecID:        "s1",
				Worktree:      "/w",
				FailureReason: "timeout",
			},
			expectedSnakeCase: []string{"schema_version", "spec_id", "failure_reason"},
		},
		{
			name: "BeadStartedEvent",
			event: BeadStartedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBeadStarted},
				BeadID:    "b1",
				BeadTitle: "t",
				Iteration: 1,
			},
			expectedSnakeCase: []string{"bead_id", "bead_title"},
		},
		{
			name: "StageStartedEvent",
			event: StageStartedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageStarted},
				StageName: "build",
				BeadID:    "b1",
				Iteration: 1,
			},
			expectedSnakeCase: []string{"stage_name", "bead_id"},
		},
		{
			name: "StageFailedEvent",
			event: StageFailedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageFailed},
				StageName: "build",
				BeadID:    "b1",
				Iteration: 1,
				Error:     "exit 1",
			},
			expectedSnakeCase: []string{"stage_name", "bead_id"},
		},
		{
			name: "StageRetryingEvent",
			event: StageRetryingEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeStageRetrying},
				StageName: "build",
				BeadID:    "b1",
				Attempt:   2,
				Reason:    "escalating",
			},
			expectedSnakeCase: []string{"stage_name", "bead_id"},
		},
		{
			name: "ScopeEvent",
			event: ScopeEvent{
				Event:      Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeScope},
				BeadID:     "b1",
				Complexity: "P1",
				Approved:   true,
				Reason:     "clear spec",
			},
			expectedSnakeCase: []string{"bead_id"},
		},
		{
			name: "TriageStartedEvent",
			event: TriageStartedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeTriageStarted},
				BeadID:    "b1",
				BeadTitle: "t",
				Iteration: 1,
			},
			expectedSnakeCase: []string{"bead_id", "bead_title"},
		},
		{
			name: "TriageCompletedEvent",
			event: TriageCompletedEvent{
				Event:     Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeTriageCompleted},
				BeadID:    "b1",
				BeadTitle: "t",
				Iteration: 1,
				Category:  "bug-fix",
				Reasoning: "timing issue",
			},
			expectedSnakeCase: []string{"bead_id", "bead_title"},
		},
		{
			name: "BuildInvocationStartEvent",
			event: BuildInvocationStartEvent{
				Event:       Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBuildInvocationStart},
				BeadID:      "b1",
				Model:       "m",
				Provider:    "p",
				Tier:        "P1",
				Attempt:     1,
				MaxAttempts: 3,
			},
			expectedSnakeCase: []string{"bead_id", "max_attempts"},
		},
		{
			name: "BuildInvocationCompleteEvent",
			event: BuildInvocationCompleteEvent{
				Event:        Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeBuildInvocationComplete},
				BeadID:       "b1",
				Model:        "m",
				Provider:     "p",
				Success:      true,
				Duration:     time.Second,
				CostUSD:      0.01,
				InputTokens:  100,
				OutputTokens: 50,
				PromptSize:   5000,
			},
			expectedSnakeCase: []string{"bead_id", "cost_usd", "input_tokens", "output_tokens", "prompt_size"},
		},
		{
			name: "ModelSelectedEvent",
			event: ModelSelectedEvent{
				Event:    Event{SchemaVersion: SchemaVersion, Timestamp: now, Type: EventTypeModelSelected},
				BeadID:   "b1",
				Model:    "m",
				Provider: "p",
				Tier:     "P1",
				Reason:   "complexity",
			},
			expectedSnakeCase: []string{"bead_id"},
		},
	}

	for _, tc := range events {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.event)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Verify expected snake_case fields are present.
			jsonStr := string(data)
			for _, field := range tc.expectedSnakeCase {
				if !strings.Contains(jsonStr, `"`+field+`"`) {
					t.Errorf("expected snake_case field %q not found in JSON: %s", field, jsonStr)
				}
			}

			// Verify no camelCase keys appear. Parse into a generic map and
			// check every key for interior uppercase letters.
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("Unmarshal to map failed: %v", err)
			}
			for key := range raw {
				if containsCamelCase(key) {
					t.Errorf("field %q appears to use camelCase instead of snake_case", key)
				}
			}
		})
	}
}

// containsCamelCase returns true if s contains a lowercase letter immediately
// followed by an uppercase letter (e.g., "beadId"), which indicates camelCase.
func containsCamelCase(s string) bool {
	prev := rune(0)
	for _, r := range s {
		if unicode.IsLower(prev) && unicode.IsUpper(r) {
			return true
		}
		prev = r
	}
	return false
}

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		event    TypedEvent
		expected string
	}{
		{BuildInvocationStartEvent{}, EventTypeBuildInvocationStart},
		{BuildInvocationCompleteEvent{}, EventTypeBuildInvocationComplete},
		{ModelSelectedEvent{}, EventTypeModelSelected},
	}
	for _, tc := range tests {
		if got := tc.event.EventType(); got != tc.expected {
			t.Errorf("EventType() = %q, want %q", got, tc.expected)
		}
	}
}

func expectEmbeddedEvent(t *testing.T, typ reflect.Type) {
	t.Helper()
	field, ok := typ.FieldByName("Event")
	if !ok {
		t.Fatalf("type %s must embed Event", typ.Name())
	}
	if !field.Anonymous {
		t.Fatalf("field Event in %s must be anonymous", typ.Name())
	}
}
