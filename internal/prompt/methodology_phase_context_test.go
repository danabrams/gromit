package prompt

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/learnings"
)

func testMethodologyContext() *Context {
	return &Context{
		Bead: &bead.Bead{
			ID:          "gromit-4rsh",
			Title:       "Implement phase-specific prompt shaping",
			Priority:    0,
			Description: "Preserve only high-signal context per methodology phase",
			Labels:      []string{"methodology:atdd", "spec:phase-isolated-methodology-contexts"},
			ExpectedOutputs: []string{
				"Red phase shaping includes acceptance/spec context and test-only guardrails",
				"Green phase shaping emphasizes failing-test signal and required behavior",
				"Refactor phase shaping uses diff/constraints focus and excludes redundant build payload",
			},
		},
		SpecName: "phase-isolated-methodology-contexts",
		Spec: `# Spec

## Acceptance Criteria
- Keep acceptance criteria and spec context in shaped prompts
- Preserve failure signal where phase requires it
- Preserve behavior-guardrail constraints for refactor`,
		Rules:    "## Rules\n- never change behavior in refactor\n- tests drive implementation",
		ClaudeMD: strings.Repeat("large project context\n", 80),
		ConfirmedLearnings: []learnings.Learning{
			{Category: "patterns", Content: "Long historical learning A"},
			{Category: "patterns", Content: "Long historical learning B"},
		},
		RecentLearnings: []learnings.Learning{
			{Category: "patterns", Content: "Recent learning C"},
		},
		RecentValidationFailures: []string{
			"--- FAIL: TestBehaviorContract (0.01s)",
			"Expected status code 422, got 200",
		},
		PrevFailure:    "--- FAIL: TestBehaviorContract (0.01s)\nexpected 422, got 200",
		FailureContext: "Do not change public behavior while implementing required acceptance criteria",
	}
}

func invokePhaseShaper(t *testing.T, r *Renderer, methodName string, ctx *Context) *Context {
	t.Helper()

	method := reflect.ValueOf(r).MethodByName(methodName)
	if !method.IsValid() {
		t.Fatalf("Renderer.%s method not found", methodName)
	}

	methodType := method.Type()
	if methodType.NumIn() != 1 || methodType.In(0) != reflect.TypeOf(&Context{}) {
		t.Fatalf("Renderer.%s should accept exactly one *Context argument", methodName)
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
	switch len(results) {
	case 1:
		if results[0].Type() != reflect.TypeOf(&Context{}) {
			t.Fatalf("Renderer.%s should return *Context", methodName)
		}
		if results[0].IsNil() {
			t.Fatalf("Renderer.%s returned nil *Context", methodName)
		}
		return results[0].Interface().(*Context)
	case 2:
		if results[0].Type() != reflect.TypeOf(&Context{}) {
			t.Fatalf("Renderer.%s first return value should be *Context", methodName)
		}
		if !results[1].IsNil() {
			err, ok := results[1].Interface().(error)
			if !ok {
				t.Fatalf("Renderer.%s second return value should be error", methodName)
			}
			t.Fatalf("Renderer.%s returned error: %v", methodName, err)
		}
		if results[0].IsNil() {
			t.Fatalf("Renderer.%s returned nil *Context", methodName)
		}
		return results[0].Interface().(*Context)
	default:
		t.Fatalf("Renderer.%s should return *Context or (*Context, error), got %d return values", methodName, len(results))
		return nil
	}
}

func TestMethodologyPhaseContextShaping_RedGreenRefactor(t *testing.T) {
	tests := []struct {
		name          string
		methodName    string
		assertShaping func(t *testing.T, shaped *Context)
	}{
		{
			name:       "red preserves acceptance and spec while trimming low-signal payload",
			methodName: "ShapeRedPhaseContext",
			assertShaping: func(t *testing.T, shaped *Context) {
				t.Helper()

				if shaped.Rules == "" {
					t.Error("red phase should preserve RULES.md context")
				}
				if shaped.Spec == "" || shaped.SpecName == "" {
					t.Error("red phase should preserve specification context")
				}
				if len(shaped.Bead.ExpectedOutputs) == 0 {
					t.Error("red phase should preserve acceptance criteria from bead expected outputs")
				}
				guardrails := strings.ToLower(fmt.Sprintf("%s\n%s", shaped.FailureContext, shaped.PrevFailure))
				if !strings.Contains(guardrails, "test-only") {
					t.Error("red phase should include test-only guardrails")
				}
				if !strings.Contains(guardrails, "implementation") {
					t.Error("red phase guardrails should prohibit implementation changes")
				}
				if shaped.ClaudeMD != "" {
					t.Error("red phase should trim low-signal project history payload (ClaudeMD)")
				}
				if len(shaped.ConfirmedLearnings) != 0 || len(shaped.RecentLearnings) != 0 {
					t.Error("red phase should trim historical learnings to keep prompt minimal")
				}
			},
		},
		{
			name:       "green emphasizes failing-test signal and required behavior",
			methodName: "ShapeGreenPhaseContext",
			assertShaping: func(t *testing.T, shaped *Context) {
				t.Helper()

				if len(shaped.Bead.ExpectedOutputs) == 0 {
					t.Error("green phase should preserve required behavior expectations")
				}
				failureSignal := strings.ToLower(fmt.Sprintf("%s\n%s", shaped.PrevFailure, strings.Join(shaped.RecentValidationFailures, "\n")))
				if !strings.Contains(failureSignal, "fail") {
					t.Error("green phase should preserve failing-test signal")
				}
				if shaped.Spec == "" {
					t.Error("green phase should preserve spec context for required behavior")
				}
				if shaped.ClaudeMD != "" {
					t.Error("green phase should exclude redundant full-history payload")
				}
				if len(shaped.ConfirmedLearnings) != 0 {
					t.Error("green phase should avoid full-history duplication from confirmed learnings")
				}
			},
		},
		{
			name:       "refactor focuses on constraints and excludes redundant build payload",
			methodName: "ShapeRefactorPhaseContext",
			assertShaping: func(t *testing.T, shaped *Context) {
				t.Helper()

				if shaped.Rules == "" {
					t.Error("refactor phase should preserve constraint context from RULES.md")
				}
				if shaped.Spec == "" {
					t.Error("refactor phase should preserve spec constraints")
				}
				guardrails := strings.ToLower(fmt.Sprintf("%s\n%s", shaped.FailureContext, shaped.PrevFailure))
				if !strings.Contains(guardrails, "do not change") && !strings.Contains(guardrails, "preserve") {
					t.Error("refactor phase should preserve behavior-preservation guardrails")
				}
				if len(shaped.RecentValidationFailures) != 0 {
					t.Error("refactor phase should exclude redundant build/test failure payload")
				}
				if len(shaped.Bead.ExpectedOutputs) != 0 {
					t.Error("refactor phase should exclude redundant build acceptance payload")
				}
				if shaped.ClaudeMD != "" {
					t.Error("refactor phase should drop broad project context and focus on constraints")
				}
			},
		},
	}

	r := &Renderer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Expected failure: Renderer.ShapeRedPhaseContext / Renderer.ShapeGreenPhaseContext / Renderer.ShapeRefactorPhaseContext are not implemented yet.
			base := testMethodologyContext()
			shaped := invokePhaseShaper(t, r, tt.methodName, base)
			tt.assertShaping(t, shaped)
		})
	}
}

func TestShapeRedPhaseContext_PreservesSignalAndTrimsNoise(t *testing.T) {
	r := &Renderer{}
	base := testMethodologyContext()

	shaped := invokePhaseShaper(t, r, "ShapeRedPhaseContext", base)

	if shaped.Rules == "" {
		t.Fatal("expected RULES.md content to be preserved")
	}
	if shaped.Spec == "" || shaped.SpecName == "" {
		t.Fatal("expected specification context to be preserved")
	}
	if len(shaped.Bead.ExpectedOutputs) == 0 {
		t.Fatal("expected acceptance criteria to be preserved")
	}
	guardrails := strings.ToLower(shaped.FailureContext + "\n" + shaped.PrevFailure)
	if !strings.Contains(guardrails, "test-only") || !strings.Contains(guardrails, "implementation") {
		t.Fatal("expected red-phase guardrails to enforce test-only changes")
	}
	if shaped.ClaudeMD != "" {
		t.Fatal("expected low-signal project history to be trimmed")
	}
	if len(shaped.ConfirmedLearnings) != 0 || len(shaped.RecentLearnings) != 0 {
		t.Fatal("expected learning history to be trimmed")
	}
}

func TestShapeGreenPhaseContext_PreservesFailureSignalAndRequiredBehavior(t *testing.T) {
	r := &Renderer{}
	base := testMethodologyContext()

	shaped := invokePhaseShaper(t, r, "ShapeGreenPhaseContext", base)

	if len(shaped.Bead.ExpectedOutputs) == 0 {
		t.Fatal("expected required behavior to be preserved")
	}
	if shaped.Spec == "" {
		t.Fatal("expected spec context to be preserved")
	}
	failureSignal := strings.ToLower(shaped.PrevFailure + "\n" + strings.Join(shaped.RecentValidationFailures, "\n"))
	if !strings.Contains(failureSignal, "fail") {
		t.Fatal("expected failing-test signal to be preserved")
	}
	if shaped.ClaudeMD != "" {
		t.Fatal("expected full-history context to be trimmed")
	}
	if len(shaped.ConfirmedLearnings) != 0 {
		t.Fatal("expected confirmed learnings to be trimmed")
	}
}

func TestShapeRefactorPhaseContext_PreservesGuardrailsAndDropsBuildPayload(t *testing.T) {
	r := &Renderer{}
	base := testMethodologyContext()

	shaped := invokePhaseShaper(t, r, "ShapeRefactorPhaseContext", base)

	if shaped.Rules == "" {
		t.Fatal("expected RULES.md constraints to be preserved")
	}
	if shaped.Spec == "" {
		t.Fatal("expected spec constraints to be preserved")
	}
	guardrails := strings.ToLower(shaped.FailureContext + "\n" + shaped.PrevFailure)
	if !strings.Contains(guardrails, "do not change") && !strings.Contains(guardrails, "preserve") {
		t.Fatal("expected behavior-preservation guardrails to be preserved")
	}
	if len(shaped.RecentValidationFailures) != 0 {
		t.Fatal("expected redundant build failures to be removed")
	}
	if len(shaped.Bead.ExpectedOutputs) != 0 {
		t.Fatal("expected redundant acceptance payload to be removed")
	}
	if shaped.ClaudeMD != "" {
		t.Fatal("expected broad project history to be trimmed")
	}
}

func TestShapeRefactorPhaseContext_AddsBehaviorPreservationGuardrailPhrase(t *testing.T) {
	r := &Renderer{}
	base := testMethodologyContext()
	base.FailureContext = ""
	base.PrevFailure = ""

	shaped := invokePhaseShaper(t, r, "ShapeRefactorPhaseContext", base)
	guardrails := shaped.FailureContext + "\n" + shaped.PrevFailure
	if !strings.Contains(guardrails, "Do not change behavior") {
		t.Fatal("expected refactor shaping to include exact behavior-preservation phrase")
	}
}
