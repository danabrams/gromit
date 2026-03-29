package stages

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/next/contract"
	"github.com/danabrams/gromit/internal/next/runstore"
	"github.com/danabrams/gromit/internal/next/specloop"
	"github.com/danabrams/gromit/internal/next/validator"
	"gopkg.in/yaml.v3"
)

type scenarioRecordingContractEvaluator struct {
	Received []*contract.ScenarioContract
}

func (r *scenarioRecordingContractEvaluator) Evaluate(_ context.Context, sc *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
	r.Received = append(r.Received, sc)
	return nil, nil
}

func TestScenario_BehavioralContractReplacesBrittleFileContains(t *testing.T) {
	// Seed
	storeDir := t.TempDir()
	store := runstore.NewStore(storeDir)
	seed := runstore.NewRunState("spec-seed", "proj-seed")
	if err := store.Save(seed); err != nil {
		t.Fatalf("seed runstore save: %v", err)
	}

	evidenceDir := t.TempDir()
	contractYAML := `scenarios:
  - name: Behavioral contract replaces brittle file_contains
    assertions:
      - file_contains:
          path: internal/next/specloop/stages/plan.go
          pattern: rework_vision_change
      - file_contains:
          path: internal/next/specloop/stages/plan.go
          pattern: rework_vision_change
`
	if err := os.WriteFile(filepath.Join(evidenceDir, "scenario-contracts.yaml"), []byte(contractYAML), 0o644); err != nil {
		t.Fatalf("write contract file: %v", err)
	}

	workDir := newValidateWorkDir(t)
	testFile := filepath.Join(workDir, "internal", "next", "specloop", "stages", "plan_scenario_behavioral_contract_replaces_brittle_file_contains_test.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0o755); err != nil {
		t.Fatalf("mkdir scenario test dir: %v", err)
	}
	testSrc := "package stages\n\nimport \"testing\"\n\nfunc TestScenario_BehavioralContractReplacesBrittleFileContains(t *testing.T) {}\n"
	if err := os.WriteFile(testFile, []byte(testSrc), 0o644); err != nil {
		t.Fatalf("write scenario test file: %v", err)
	}

	v := &fakeValidator{result: validator.FinalResult{Pass: true}}
	evaluator := &scenarioRecordingContractEvaluator{}
	stage := NewValidateStage(v, ValidateStageConfig{
		WorkDir:     workDir,
		EvidenceDir: evidenceDir,
	}, nil, evaluator, nil)

	rs := runstore.NewRunState("spec-001", "proj-001")

	// Invoke
	action, err := stage.Run(context.Background(), rs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Kind != specloop.Continue {
		t.Fatalf("expected Continue action, got %v", action.Kind)
	}

	// Assert
	if len(evaluator.Received) != 1 {
		t.Fatalf("expected evaluator to receive one contract, got %d", len(evaluator.Received))
	}
	if len(evaluator.Received[0].Scenarios) != 1 {
		t.Fatalf("expected one scenario, got %d", len(evaluator.Received[0].Scenarios))
	}

	assertions := evaluator.Received[0].Scenarios[0].Assertions
	if len(assertions) != 1 {
		t.Fatalf("expected file_contains assertions to be replaced by one assertion, got %d", len(assertions))
	}
	if assertions[0].FileContains != nil {
		t.Fatal("expected file_contains assertion to be removed")
	}

	goTestPassField := reflect.ValueOf(assertions[0]).FieldByName("GoTestPass")
	if !goTestPassField.IsValid() || goTestPassField.IsNil() {
		t.Fatal("expected replacement assertion to be go_test_pass")
	}

	pkgField := goTestPassField.Elem().FieldByName("Pkg")
	if !pkgField.IsValid() || pkgField.Kind() != reflect.String {
		t.Fatal("expected go_test_pass.pkg to be present")
	}
	if got := pkgField.String(); got != "./internal/next/specloop/stages" {
		t.Fatalf("expected pkg ./internal/next/specloop/stages/..., got %q", got)
	}

	testNameField := goTestPassField.Elem().FieldByName("TestName")
	if !testNameField.IsValid() || testNameField.Kind() != reflect.String {
		t.Fatal("expected go_test_pass.test_name to be present")
	}
	if got := testNameField.String(); got != "TestScenario_BehavioralContractReplacesBrittleFileContains" {
		t.Fatalf("expected test_name TestScenario_BehavioralContractReplacesBrittleFileContains, got %q", got)
	}

	encoded, err := yaml.Marshal(assertions[0])
	if err != nil {
		t.Fatalf("marshal assertion: %v", err)
	}
	encodedStr := string(encoded)
	if !strings.Contains(encodedStr, "go_test_pass") {
		t.Fatalf("expected marshaled assertion to contain go_test_pass, got %q", encodedStr)
	}
	if strings.Contains(encodedStr, "file_contains") {
		t.Fatalf("expected marshaled assertion to exclude file_contains, got %q", encodedStr)
	}
}
