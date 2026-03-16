diff --git a/cmd/gromit-next/stage_provider.go b/cmd/gromit-next/stage_provider.go
index f71030f5c..881aa5526 100644
--- a/cmd/gromit-next/stage_provider.go
+++ b/cmd/gromit-next/stage_provider.go
@@ -8,6 +8,7 @@ import (
 	"time"
 
 	"github.com/danabrams/gromit/internal/next/acceptor"
+	"github.com/danabrams/gromit/internal/next/contract"
 	"github.com/danabrams/gromit/internal/next/execpolicy"
 	"github.com/danabrams/gromit/internal/next/llmadapter"
 	"github.com/danabrams/gromit/internal/next/planner"
@@ -95,13 +96,14 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 	}
 
 	var (
-		compiler     stages.SpecCompiler
-		planCreator  stages.PlanCreator
-		taskRunner   specloop.TaskRunner
-		finalVal     stages.FinalValidator
-		reviewRunner stages.ReviewRunner
-		acceptEval   stages.AcceptEvaluator
-		diffProv     review.DiffProvider = &noopDiffProvider{}
+		compiler       stages.SpecCompiler
+		planCreator    stages.PlanCreator
+		contractWriter stages.ContractWriter
+		taskRunner     specloop.TaskRunner
+		finalVal       stages.FinalValidator
+		reviewRunner   stages.ReviewRunner
+		acceptEval     stages.AcceptEvaluator
+		diffProv       review.DiffProvider = &noopDiffProvider{}
 	)
 
 	if p.claudeProvider != nil {
@@ -119,6 +121,14 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		pl := planner.NewPlanner(planAgent, policy.Models.Planner)
 		planCreator = pl
 
+		// Contract writer adapter (Sonnet / P1 tier via evaluator tier).
+		contractAdapter := llmadapter.NewFallbackAdapter(
+			router, "contract",
+			llmadapter.Config{Tier: policy.Models.Evaluator, OnCost: costCallback, OnInvocation: invocationCallback},
+			policy.Models.Evaluator,
+		)
+		contractWriter = contract.NewLLMContractWriter(contractAdapter)
+
 		// Execute adapter: OnCost is intentionally nil to avoid double-counting.
 		// RunTaskLoop already calls Budget.AddCost(result.Cost) after each task,
 		// so wiring OnCost here would count execution costs twice.
@@ -168,6 +178,7 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		// Fallback to noops when no Provider is configured.
 		compiler = &noopCompiler{}
 		planCreator = &noopPlanCreator{}
+		contractWriter = &noopContractWriter{}
 		taskRunner = &noopTaskRunner{}
 		finalVal = &noopValidator{}
 		reviewRunner = &noopReviewRunner{}
@@ -210,13 +221,21 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		EventLog:               eventLog,
 	})
 
+	evidenceDir := store.RunEvidenceDir(rs.RunID)
+
+	writeContractsStage := stages.NewWriteContractsStage(contractWriter, stages.WriteContractsStageConfig{
+		Store:       store,
+		SpecPath:    p.cfg.SpecPath,
+		EvidenceDir: evidenceDir,
+	}, eventLog, budget)
+
 	validateStage := stages.NewValidateStage(finalVal, stages.ValidateStageConfig{
-		AlwaysRun: alwaysRun,
-		WorkDir:   p.cfg.WorkDir,
+		AlwaysRun:   alwaysRun,
+		WorkDir:     p.cfg.WorkDir,
+		EvidenceDir: evidenceDir,
+		Evaluator:   contract.NewFileSystemEvaluator(p.cfg.WorkDir),
 	}, nil)
 
-	evidenceDir := store.RunEvidenceDir(rs.RunID)
-
 	reviewStage := stages.NewReviewStage(reviewRunner, stages.ReviewStageConfig{
 		SpecContent:  string(specContent),
 		EvidenceDir:  evidenceDir,
@@ -247,6 +266,7 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		initStage,
 		compileStage,
 		planStage,
+		writeContractsStage,
 		executeStage,
 		validateStage,
 		reviewStage,
@@ -406,6 +426,13 @@ func (d *PlannerDecomposer) Decompose(ctx context.Context, task runstore.Task) (
 	return subTasks, nil
 }
 
+// noopContractWriter satisfies stages.ContractWriter with a no-op.
+type noopContractWriter struct{}
+
+func (n *noopContractWriter) WriteContracts(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
+	return &contract.ScenarioContract{}, nil
+}
+
 // noopAcceptEvaluator satisfies AcceptEvaluator with a no-op.
 type noopAcceptEvaluator struct{}
 
diff --git a/cmd/gromit-next/stage_provider_test.go b/cmd/gromit-next/stage_provider_test.go
index 3191ae1d3..bfe6dc521 100644
--- a/cmd/gromit-next/stage_provider_test.go
+++ b/cmd/gromit-next/stage_provider_test.go
@@ -36,7 +36,7 @@ func TestRealStageProvider_BuildStages_ReturnsStages(t *testing.T) {
 		t.Fatal("expected at least one stage, got 0")
 	}
 
-	expectedNames := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
+	expectedNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence", "finalize"}
 	if len(stages) != len(expectedNames) {
 		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
 	}
@@ -155,8 +155,8 @@ func TestRealStageProvider_BuildStages_DefaultTierUsesModelsEvaluator(t *testing
 		t.Fatalf("BuildStages: %v", err)
 	}
 	// Verify we got the expected stages (sanity check).
-	if len(stages) != 9 {
-		t.Fatalf("expected 9 stages, got %d", len(stages))
+	if len(stages) != 10 {
+		t.Fatalf("expected 10 stages, got %d", len(stages))
 	}
 }
 
@@ -452,7 +452,7 @@ func TestRealStageProvider_BuildStages_WithProvider_ReturnsRealAdapters(t *testi
 		t.Fatalf("BuildStages returned error: %v", err)
 	}
 
-	expectedNames := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
+	expectedNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence", "finalize"}
 	if len(stages) != len(expectedNames) {
 		t.Fatalf("expected %d stages, got %d", len(expectedNames), len(stages))
 	}
@@ -478,8 +478,8 @@ func TestRealStageProvider_BuildStages_WithProvider_NilProviderFallsBackToNoops(
 	if err != nil {
 		t.Fatalf("BuildStages returned error: %v", err)
 	}
-	if len(stages) != 9 {
-		t.Fatalf("expected 9 stages, got %d", len(stages))
+	if len(stages) != 10 {
+		t.Fatalf("expected 10 stages, got %d", len(stages))
 	}
 }
 
@@ -550,8 +550,8 @@ func TestBuildStages_WithClaudeProvider_UsesFallbackAdapter(t *testing.T) {
 	if err != nil {
 		t.Fatalf("BuildStages returned error: %v", err)
 	}
-	if len(stages) != 9 {
-		t.Fatalf("expected 9 stages, got %d", len(stages))
+	if len(stages) != 10 {
+		t.Fatalf("expected 10 stages, got %d", len(stages))
 	}
 }
 
@@ -898,8 +898,8 @@ func TestIntegration_BuildStages_FallbackAdapter_RouterWiring(t *testing.T) {
 	if len(stages) == 0 {
 		t.Fatal("expected at least one stage from BuildStages")
 	}
-	// Verify 9 stages are returned (same as before — multi-provider doesn't change stage count)
-	if len(stages) != 9 {
-		t.Fatalf("expected 9 stages, got %d", len(stages))
+	// Verify 10 stages are returned (write_contracts stage added between plan and execute)
+	if len(stages) != 10 {
+		t.Fatalf("expected 10 stages, got %d", len(stages))
 	}
 }
diff --git a/cmd/gromit/migration_compat_command_surface_test.go b/cmd/gromit/migration_compat_command_surface_test.go
index 634821cd6..a18b254cf 100644
--- a/cmd/gromit/migration_compat_command_surface_test.go
+++ b/cmd/gromit/migration_compat_command_surface_test.go
@@ -45,7 +45,7 @@ func TestDebugCompatibilityDiagnostics_CommandSurfaceSupportsLegacyAndExplicitCo
 	for _, tc := range tests {
 		t.Run(tc.name, func(t *testing.T) {
 			t.Parallel()
-			cfgPath := filepath.Join("..", "..", "test", "fixtures", "migration", tc.fixture)
+			cfgPath := resolveProjectPath("t", filepath.Join("test", "fixtures", "migration", tc.fixture))
 			cfg, err := config.Load(cfgPath)
 			if err != nil {
 				t.Fatalf("Load(%q) error = %v", cfgPath, err)
diff --git a/cmd/gromit/run_spec_flag_test.go b/cmd/gromit/run_spec_flag_test.go
index e4691e27b..0847b6672 100644
--- a/cmd/gromit/run_spec_flag_test.go
+++ b/cmd/gromit/run_spec_flag_test.go
@@ -117,8 +117,16 @@ func TestRunSpecTestEnvRestoresWorkingDirectory(t *testing.T) {
 	if err != nil {
 		t.Fatalf("getting working directory after cleanup: %v", err)
 	}
-	if currentWD != originalWD {
-		t.Fatalf("expected working directory %q after cleanup, got %q", originalWD, currentWD)
+	resolvedOriginal, err := filepath.EvalSymlinks(originalWD)
+	if err != nil {
+		t.Fatalf("resolving symlinks in original working directory: %v", err)
+	}
+	resolvedCurrent, err := filepath.EvalSymlinks(currentWD)
+	if err != nil {
+		t.Fatalf("resolving symlinks in current working directory: %v", err)
+	}
+	if resolvedCurrent != resolvedOriginal {
+		t.Fatalf("expected working directory %q after cleanup, got %q", resolvedOriginal, resolvedCurrent)
 	}
 }
 
diff --git a/internal/next/runstore/events.go b/internal/next/runstore/events.go
index 110997611..f0851ba7c 100644
--- a/internal/next/runstore/events.go
+++ b/internal/next/runstore/events.go
@@ -138,6 +138,16 @@ type BlockedWorktreeCleanedEvent struct {
 	WorktreePath string `json:"worktree_path"`
 }
 
+type ContractsWrittenEvent struct {
+	BaseEvent
+	ScenarioCount int `json:"scenario_count"`
+}
+
+type ContractsBlockedEvent struct {
+	BaseEvent
+	Reason string `json:"reason"`
+}
+
 type TerminalStateEvent struct {
 	BaseEvent
 	Status string `json:"status"`
@@ -274,6 +284,12 @@ func unmarshalEvent(data []byte) (TypedEvent, error) {
 	case "blocked_worktree_cleaned":
 		var e BlockedWorktreeCleanedEvent
 		ev = &e
+	case "contracts_written":
+		var e ContractsWrittenEvent
+		ev = &e
+	case "contracts_blocked":
+		var e ContractsBlockedEvent
+		ev = &e
 	case "terminal_state":
 		var e TerminalStateEvent
 		ev = &e
diff --git a/internal/next/runstore/types.go b/internal/next/runstore/types.go
index 98ac32472..446b762f6 100644
--- a/internal/next/runstore/types.go
+++ b/internal/next/runstore/types.go
@@ -36,6 +36,7 @@ type RunState struct {
 	FinalValidationPassed bool                   `json:"final_validation_passed"`
 	FinalReviewPassed     bool                   `json:"final_review_passed"`
 	FinalAcceptancePassed bool                   `json:"final_acceptance_passed"`
+	ContractsWritten      bool                   `json:"contracts_written"`
 	ReplanContext         []string               `json:"replan_context,omitempty"`
 	LastValidationResult  *string                `json:"last_validation_result,omitempty"`
 	LastFinalValidation   *validator.FinalResult `json:"last_final_validation,omitempty"`
diff --git a/internal/next/specloop/stages/validate.go b/internal/next/specloop/stages/validate.go
index 59f8915f6..332b4379d 100644
--- a/internal/next/specloop/stages/validate.go
+++ b/internal/next/specloop/stages/validate.go
@@ -3,11 +3,15 @@ package stages
 import (
 	"context"
 	"fmt"
+	"os"
+	"path/filepath"
 	"time"
 
+	"github.com/danabrams/gromit/internal/next/contract"
 	"github.com/danabrams/gromit/internal/next/runstore"
 	"github.com/danabrams/gromit/internal/next/specloop"
 	"github.com/danabrams/gromit/internal/next/validator"
+	"gopkg.in/yaml.v3"
 )
 
 // FinalValidator abstracts final validation for testability.
@@ -15,11 +19,18 @@ type FinalValidator interface {
 	RunFinal(ctx context.Context, alwaysRun []validator.Check, projectChecks []validator.Check, workDir string) (validator.FinalResult, error)
 }
 
+// ContractEvaluator abstracts scenario contract evaluation for testability.
+type ContractEvaluator interface {
+	Evaluate(ctx context.Context, contract *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error)
+}
+
 // ValidateStageConfig configures the ValidateStage.
 type ValidateStageConfig struct {
 	AlwaysRun     []validator.Check
 	ProjectChecks []validator.Check
 	WorkDir       string
+	EvidenceDir   string
+	Evaluator     ContractEvaluator
 }
 
 // ValidateStage runs final validation checks.
@@ -43,6 +54,30 @@ func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloo
 	if rs.WorktreePath != "" {
 		workDir = rs.WorktreePath
 	}
+
+	// Contract evaluation (if configured)
+	var failures []string
+	if s.cfg.Evaluator != nil && s.cfg.EvidenceDir != "" {
+		contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")
+		if _, err := os.Stat(contractPath); err == nil {
+			data, err := os.ReadFile(contractPath)
+			if err != nil {
+				return specloop.NextAction{}, fmt.Errorf("read scenario-contracts.yaml: %w", err)
+			}
+			var sc contract.ScenarioContract
+			if err := yaml.Unmarshal(data, &sc); err != nil {
+				return specloop.NextAction{}, fmt.Errorf("parse scenario-contracts.yaml: %w", err)
+			}
+			contractFailures, err := s.cfg.Evaluator.Evaluate(ctx, &sc, workDir)
+			if err != nil {
+				return specloop.NextAction{}, fmt.Errorf("contract evaluation: %w", err)
+			}
+			for _, cf := range contractFailures {
+				failures = append(failures, fmt.Sprintf("contract:%s — %s failed: %s", cf.ScenarioName, cf.AssertionType, cf.Details))
+			}
+		}
+	}
+
 	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
 	if err != nil {
 		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
@@ -61,19 +96,19 @@ func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloo
 		})
 	}
 
-	if result.Pass {
-		rs.FinalValidationPassed = true
-		return specloop.NextAction{Kind: specloop.Continue}, nil
-	}
-
-	// Collect failure details
-	var failures []string
+	// Collect shell check failures
 	for _, cr := range result.AlwaysRun.FailedChecks() {
 		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, cr.Output))
 	}
 	for _, cr := range result.ProjectChecks.FailedChecks() {
 		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, cr.Output))
 	}
+
+	if result.Pass && len(failures) == 0 {
+		rs.FinalValidationPassed = true
+		return specloop.NextAction{Kind: specloop.Continue}, nil
+	}
+
 	if len(failures) == 0 {
 		failures = []string{"validation failed"}
 	}
diff --git a/internal/next/specloop/stages/validate_test.go b/internal/next/specloop/stages/validate_test.go
index c2bf6dc73..58155d5d3 100644
--- a/internal/next/specloop/stages/validate_test.go
+++ b/internal/next/specloop/stages/validate_test.go
@@ -2,8 +2,12 @@ package stages
 
 import (
 	"context"
+	"os"
+	"path/filepath"
+	"strings"
 	"testing"
 
+	"github.com/danabrams/gromit/internal/next/contract"
 	"github.com/danabrams/gromit/internal/next/runstore"
 	"github.com/danabrams/gromit/internal/next/specloop"
 	"github.com/danabrams/gromit/internal/next/validator"
@@ -18,6 +22,58 @@ func (f *fakeValidator) RunFinal(ctx context.Context, alwaysRun []validator.Chec
 	return f.result, f.err
 }
 
+type fakeContractEvaluator struct {
+	failures []contract.ContractFailure
+	err      error
+	called   bool
+}
+
+func (f *fakeContractEvaluator) Evaluate(ctx context.Context, sc *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
+	f.called = true
+	return f.failures, f.err
+}
+
+// passingValidator returns a fakeValidator that reports all checks passed.
+func passingValidator() *fakeValidator {
+	return &fakeValidator{
+		result: validator.FinalResult{
+			Pass: true,
+			AlwaysRun: validator.CheckResults{
+				Results: []validator.CheckResult{{Name: "test", Pass: true}},
+			},
+			ProjectChecks: validator.CheckResults{
+				Results: []validator.CheckResult{{Name: "lint", Pass: true}},
+			},
+		},
+	}
+}
+
+// failingValidator returns a fakeValidator that reports a shell check failure.
+func failingValidator() *fakeValidator {
+	return &fakeValidator{
+		result: validator.FinalResult{
+			Pass: false,
+			AlwaysRun: validator.CheckResults{
+				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "shell fail"}},
+			},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+}
+
+// writeContractFile writes a minimal scenario-contracts.yaml to dir and returns dir.
+func writeContractFile(t *testing.T, dir string) {
+	t.Helper()
+	yaml := `scenarios:
+  - name: my-scenario
+    assertions:
+      - file_exists: some/path.txt
+`
+	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(yaml), 0o644); err != nil {
+		t.Fatalf("write scenario-contracts.yaml: %v", err)
+	}
+}
+
 // Verify ValidateStage satisfies the Stage interface.
 var _ specloop.Stage = (*ValidateStage)(nil)
 
@@ -87,3 +143,173 @@ func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
 		t.Fatal("expected failures to be non-empty")
 	}
 }
+
+func TestValidate_ContractFailure(t *testing.T) {
+	dir := t.TempDir()
+	writeContractFile(t, dir)
+
+	evaluator := &fakeContractEvaluator{
+		failures: []contract.ContractFailure{
+			{ScenarioName: "my-scenario", AssertionType: "file_exists", Details: "path not found"},
+		},
+	}
+
+	stage := NewValidateStage(passingValidator(), ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+		Evaluator:   evaluator,
+	}, nil)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.ReplanFrom {
+		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
+	}
+	if action.Context == nil || len(action.Context.Failures) == 0 {
+		t.Fatal("expected non-empty failures")
+	}
+	got := action.Context.Failures[0]
+	// Expected format: "contract:<name> — <type> failed: <details>"
+	want := "contract:my-scenario — file_exists failed: path not found"
+	if got != want {
+		t.Errorf("failure = %q, want %q", got, want)
+	}
+}
+
+func TestValidate_ContractAndShellBothFail(t *testing.T) {
+	dir := t.TempDir()
+	writeContractFile(t, dir)
+
+	evaluator := &fakeContractEvaluator{
+		failures: []contract.ContractFailure{
+			{ScenarioName: "my-scenario", AssertionType: "file_exists", Details: "not found"},
+		},
+	}
+
+	stage := NewValidateStage(failingValidator(), ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+		Evaluator:   evaluator,
+	}, nil)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.ReplanFrom {
+		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
+	}
+	if action.Context == nil {
+		t.Fatal("expected FailureContext to be non-nil")
+	}
+
+	var hasContract, hasShell bool
+	for _, f := range action.Context.Failures {
+		if strings.Contains(f, "contract:") {
+			hasContract = true
+		}
+		if strings.Contains(f, "shell fail") {
+			hasShell = true
+		}
+	}
+	if !hasContract {
+		t.Errorf("expected contract failure in %v", action.Context.Failures)
+	}
+	if !hasShell {
+		t.Errorf("expected shell failure in %v", action.Context.Failures)
+	}
+}
+
+func TestValidate_ContractFileMissing(t *testing.T) {
+	dir := t.TempDir()
+	// No scenario-contracts.yaml written.
+
+	evaluator := &fakeContractEvaluator{}
+
+	stage := NewValidateStage(passingValidator(), ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+		Evaluator:   evaluator,
+	}, nil)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+	if evaluator.called {
+		t.Error("expected evaluator NOT to be called when contract file is absent")
+	}
+}
+
+func TestValidate_ContractPassShellFail(t *testing.T) {
+	dir := t.TempDir()
+	writeContractFile(t, dir)
+
+	evaluator := &fakeContractEvaluator{
+		failures: nil, // contracts pass
+	}
+
+	stage := NewValidateStage(failingValidator(), ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+		Evaluator:   evaluator,
+	}, nil)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.ReplanFrom {
+		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
+	}
+	if action.Context == nil || len(action.Context.Failures) == 0 {
+		t.Fatal("expected non-empty failures")
+	}
+	for _, f := range action.Context.Failures {
+		if strings.Contains(f, "contract:") {
+			t.Errorf("unexpected contract failure when contracts pass: %q", f)
+		}
+	}
+	var hasShell bool
+	for _, f := range action.Context.Failures {
+		if strings.Contains(f, "shell fail") {
+			hasShell = true
+		}
+	}
+	if !hasShell {
+		t.Errorf("expected shell failure in %v", action.Context.Failures)
+	}
+}
+
+func TestValidate_NilEvaluator(t *testing.T) {
+	dir := t.TempDir()
+	writeContractFile(t, dir)
+
+	// Evaluator is nil — contract check should be skipped entirely.
+	stage := NewValidateStage(passingValidator(), ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+		Evaluator:   nil,
+	}, nil)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+	if !rs.FinalValidationPassed {
+		t.Fatal("expected FinalValidationPassed to be true")
+	}
+}