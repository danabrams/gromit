# Review Decision Sheet

## Terminal State

needs_human

## What Changed

diff --git a/cmd/gromit-next/exec.go b/cmd/gromit-next/exec.go
index 1be728841..64cabdaf1 100644
--- a/cmd/gromit-next/exec.go
+++ b/cmd/gromit-next/exec.go
@@ -54,6 +54,27 @@ var dryRunStages = map[string]bool{
 	"plan":    true,
 }
 
+// filterStagesForResume returns stages with compile removed,
+// since that stage has already been completed in the prior run.
+// When worktreePath is non-empty, init is also skipped because the worktree
+// already exists from the prior run.
+// Other stages run idempotently based on their corresponding completion flags.
+func filterStagesForResume(stages []specloop.Stage, worktreePath string) []specloop.Stage {
+	var filtered []specloop.Stage
+	for _, s := range stages {
+		switch s.Name() {
+		case "compile":
+			continue
+		case "init":
+			if worktreePath != "" {
+				continue
+			}
+		}
+		filtered = append(filtered, s)
+	}
+	return filtered
+}
+
 // filterStagesForDryRun returns only the dry-run stages when dryRun is true,
 // or all stages when dryRun is false.
 func filterStagesForDryRun(stages []specloop.Stage, dryRun bool) []specloop.Stage {
@@ -69,26 +90,6 @@ func filterStagesForDryRun(stages []specloop.Stage, dryRun bool) []specloop.Stag
 	return filtered
 }
 
-// filterStagesForResume removes stages that are unnecessary when resuming
-// a prior run. Init is skipped if a worktree already exists, compile is
-// always skipped. Plan is kept in the list (specloop needs it for replan
-// jumps) but will no-op when tasks exist and it's not a fix cycle.
-func filterStagesForResume(stages []specloop.Stage, rs *runstore.RunState) []specloop.Stage {
-	var filtered []specloop.Stage
-	for _, s := range stages {
-		switch s.Name() {
-		case "init":
-			if rs.WorktreePath != "" {
-				continue
-			}
-		case "compile":
-			continue
-		}
-		filtered = append(filtered, s)
-	}
-	return filtered
-}
-
 // StageProvider builds the ordered set of stages for an exec spec run.
 // Implementations wire real or test dependencies into each stage.
 // The Budget parameter is the single shared instance that tracks cost and time
@@ -160,7 +161,11 @@ func (e *execSpecRun) run(ctx context.Context) (string, error) {
 		rs = runstore.NewRunState(specIDFromPath(e.specPath), e.projectID)
 	}
 
-	// 3. Create a single shared Budget instance.
+	// 3. Create a single shared Budget instance. This same instance is passed
+	// to both the SpecLoop (for cycle counting and hard budget checks between
+	// stages) and to ExecuteStage (for per-task cost accumulation). Using one
+	// instance ensures cost tracked during task execution is visible to the
+	// SpecLoop's budget gate.
 	// On resume, override MaxSpecCycles with the requested cycle count.
 	if e.resumeRunID != "" && e.resumeCycles > 0 {
 		policy.Budgets.MaxSpecCycles = e.resumeCycles
@@ -180,7 +185,7 @@ func (e *execSpecRun) run(ctx context.Context) (string, error) {
 	// 5. Filter for dry-run or resume
 	stages = filterStagesForDryRun(stages, e.dryRun)
 	if e.resumeRunID != "" {
-		stages = filterStagesForResume(stages, rs)
+		stages = filterStagesForResume(stages, rs.WorktreePath)
 	}
 	loop := specloop.NewSpecLoop(stages, specloop.SpecLoopConfig{
 		Budget:      budget,
diff --git a/cmd/gromit-next/exec_test.go b/cmd/gromit-next/exec_test.go
index 1690ef179..0612066ad 100644
--- a/cmd/gromit-next/exec_test.go
+++ b/cmd/gromit-next/exec_test.go
@@ -629,6 +629,43 @@ func TestFilterStagesForDryRun_ExcludesReviewAndAccept(t *testing.T) {
 	}
 }
 
+// TestFilterStagesForResume_SkipsCompile verifies that compile is skipped
+// when resuming a run (it has already been completed in the prior run).
+func TestFilterStagesForResume_SkipsCompile(t *testing.T) {
+	allNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence", "finalize"}
+	var allStages []specloop.Stage
+	for _, name := range allNames {
+		allStages = append(allStages, &stageRecorder{name: name})
+	}
+
+	filtered := filterStagesForResume(allStages, "")
+
+	for _, s := range filtered {
+		if s.Name() == "compile" {
+			t.Errorf("resume should skip %q stage (already done in prior run)", s.Name())
+		}
+	}
+	wantCount := len(allNames) - 1 // only compile skipped
+	if len(filtered) != wantCount {
+		t.Errorf("expected %d stages after resume filter, got %d", wantCount, len(filtered))
+	}
+}
+
+// TestFilterStagesForResume_KeepsOtherStages verifies that all other stages are kept.
+func TestFilterStagesForResume_KeepsOtherStages(t *testing.T) {
+	allNames := []string{"init", "plan", "execute", "validate", "review", "accept", "evidence", "finalize"}
+	var allStages []specloop.Stage
+	for _, name := range allNames {
+		allStages = append(allStages, &stageRecorder{name: name})
+	}
+
+	filtered := filterStagesForResume(allStages, "")
+
+	if len(filtered) != len(allNames) {
+		t.Errorf("expected all %d stages kept (compile not in input to skip), got %d", len(allNames), len(filtered))
+	}
+}
+
 // Test that exec show returns a friendly error for an unknown run ID.
 func TestExecShowCmd_UnknownRunID_FriendlyError(t *testing.T) {
 	tmp := t.TempDir()
diff --git a/cmd/gromit-next/resume_contract_test.go b/cmd/gromit-next/resume_contract_test.go
index 681deef73..29d86fbcd 100644
--- a/cmd/gromit-next/resume_contract_test.go
+++ b/cmd/gromit-next/resume_contract_test.go
@@ -108,6 +108,12 @@ func TestResumeContract_ResumedRunReusesWorktree(t *testing.T) {
 					stagesRun = append(stagesRun, "compile")
 				},
 			},
+			&stageRecorderFunc{
+				name: "write_contracts",
+				fn: func(rs *runstore.RunState) {
+					stagesRun = append(stagesRun, "write_contracts")
+				},
+			},
 			&stageRecorderFunc{
 				name: "execute",
 				fn: func(rs *runstore.RunState) {
@@ -136,10 +142,14 @@ func TestResumeContract_ResumedRunReusesWorktree(t *testing.T) {
 		t.Errorf("expected worktree %q, got %q", "/tmp/original-worktree", capturedWT)
 	}
 
-	// init stage must NOT have run (filtered out because worktree exists)
+	// init and compile must NOT have run (filtered out on resume)
+	// write_contracts should run (relies on ContractsWritten flag for idempotency)
 	for _, name := range stagesRun {
 		if name == "init" {
-			t.Error("init stage should not run on resume when worktree exists")
+			t.Error("init stage should not run on resume")
+		}
+		if name == "compile" {
+			t.Error("compile stage should not run on resume")
 		}
 	}
 }
@@ -220,10 +230,6 @@ func TestResumeContract_ResumedRunIncludesPlanStage(t *testing.T) {
 	var stagesRun []string
 	provider := &testStageProvider{
 		stages: []specloop.Stage{
-			&stageRecorderFunc{
-				name: "init",
-				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "init") },
-			},
 			&stageRecorderFunc{
 				name: "compile",
 				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "compile") },
@@ -232,6 +238,10 @@ func TestResumeContract_ResumedRunIncludesPlanStage(t *testing.T) {
 				name: "plan",
 				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "plan") },
 			},
+			&stageRecorderFunc{
+				name: "write_contracts",
+				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "write_contracts") },
+			},
 			&stageRecorderFunc{
 				name: "execute",
 				fn:   func(_ *runstore.RunState) { stagesRun = append(stagesRun, "execute") },
@@ -262,6 +272,14 @@ func TestResumeContract_ResumedRunIncludesPlanStage(t *testing.T) {
 	if !planRan {
 		t.Error("plan stage should be included on resume (needed for replan cycles)")
 	}
+
+	// compile must not have run (filtered out on resume)
+	// write_contracts should run (relies on ContractsWritten flag for idempotency)
+	for _, name := range stagesRun {
+		if name == "compile" {
+			t.Error("compile should not run on resume")
+		}
+	}
 }
 
 func TestResumeContract_GateFlagsResetOnResume(t *testing.T) {
@@ -371,9 +389,7 @@ func TestResumeScenario_HumanSaysKeepGoing(t *testing.T) {
 			},
 			&stageRecorderFunc{
 				name: "validate",
-				fn: func(rs *runstore.RunState) {
-					// Simulate: validation fails, triggering replan
-				},
+				fn:   func(_ *runstore.RunState) {},
 			},
 		},
 	}
diff --git a/cmd/gromit-next/resume_test.go b/cmd/gromit-next/resume_test.go
index 10944e516..b605579d8 100644
--- a/cmd/gromit-next/resume_test.go
+++ b/cmd/gromit-next/resume_test.go
@@ -13,31 +13,36 @@ import (
 )
 
 func TestFilterStagesForResume(t *testing.T) {
-	allNames := []string{"init", "compile", "plan", "execute", "validate", "review", "accept", "evidence"}
+	allNames := []string{"init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence"}
 	var allStages []specloop.Stage
 	for _, name := range allNames {
 		allStages = append(allStages, &stageRecorder{name: name})
 	}
 
-	t.Run("skips init when worktree exists", func(t *testing.T) {
-		rs := &runstore.RunState{
-			WorktreePath: "/tmp/wt",
-			Tasks:        []runstore.Task{{TaskID: "t1", Status: "done"}},
-		}
-		filtered := filterStagesForResume(allStages, rs)
+	t.Run("always skips compile", func(t *testing.T) {
+		filtered := filterStagesForResume(allStages, "")
 		for _, s := range filtered {
-			if s.Name() == "init" {
-				t.Fatal("init should be skipped when WorktreePath is set")
+			if s.Name() == "compile" {
+				t.Fatal("compile should always be skipped on resume")
 			}
 		}
 	})
 
-	t.Run("keeps init when no worktree", func(t *testing.T) {
-		rs := &runstore.RunState{
-			WorktreePath: "",
-			Tasks:        []runstore.Task{{TaskID: "t1", Status: "done"}},
+	t.Run("keeps write_contracts (runs idempotently via ContractsWritten flag)", func(t *testing.T) {
+		filtered := filterStagesForResume(allStages, "")
+		found := false
+		for _, s := range filtered {
+			if s.Name() == "write_contracts" {
+				found = true
+			}
+		}
+		if !found {
+			t.Fatal("write_contracts should be kept (relies on ContractsWritten flag for idempotency)")
 		}
-		filtered := filterStagesForResume(allStages, rs)
+	})
+
+	t.Run("keeps init when no WorktreePath", func(t *testing.T) {
+		filtered := filterStagesForResume(allStages, "")
 		found := false
 		for _, s := range filtered {
 			if s.Name() == "init" {
@@ -45,25 +50,21 @@ func TestFilterStagesForResume(t *testing.T) {
 			}
 		}
 		if !found {
-			t.Fatal("init should be kept when WorktreePath is empty")
+			t.Fatal("init should be kept on resume when no WorktreePath is set")
 		}
 	})
 
-	t.Run("always skips compile", func(t *testing.T) {
-		rs := &runstore.RunState{}
-		filtered := filterStagesForResume(allStages, rs)
+	t.Run("init should be skipped when WorktreePath is set", func(t *testing.T) {
+		filtered := filterStagesForResume(allStages, "/tmp/existing-worktree")
 		for _, s := range filtered {
-			if s.Name() == "compile" {
-				t.Fatal("compile should always be skipped on resume")
+			if s.Name() == "init" {
+				t.Fatal("init should be skipped when WorktreePath is set")
 			}
 		}
 	})
 
 	t.Run("keeps plan in stage list for replan jumps", func(t *testing.T) {
-		rs := &runstore.RunState{
-			Tasks: []runstore.Task{{TaskID: "t1", Status: "done"}},
-		}
-		filtered := filterStagesForResume(allStages, rs)
+		filtered := filterStagesForResume(allStages, "")
 		found := false
 		for _, s := range filtered {
 			if s.Name() == "plan" {
@@ -76,11 +77,7 @@ func TestFilterStagesForResume(t *testing.T) {
 	})
 
 	t.Run("keeps execute validate review accept evidence", func(t *testing.T) {
-		rs := &runstore.RunState{
-			WorktreePath: "/tmp/wt",
-			Tasks:        []runstore.Task{{TaskID: "t1"}},
-		}
-		filtered := filterStagesForResume(allStages, rs)
+		filtered := filterStagesForResume(allStages, "")
 		names := make(map[string]bool)
 		for _, s := range filtered {
 			names[s.Name()] = true
@@ -117,6 +114,7 @@ func TestExecSpec_ResumeLoadsExistingRunState(t *testing.T) {
 			&stageRecorder{name: "init", orderPtr: &order},
 			&stageRecorder{name: "compile", orderPtr: &order},
 			&stageRecorder{name: "plan", orderPtr: &order},
+			&stageRecorder{name: "write_contracts", orderPtr: &order},
 			&stageRecorder{name: "execute", orderPtr: &order},
 			&stageRecorder{name: "validate", orderPtr: &order},
 		},
@@ -153,7 +151,7 @@ func TestExecSpec_ResumeLoadsExistingRunState(t *testing.T) {
 	}
 }
 
-func TestExecSpec_ResumeSkipsInitCompile(t *testing.T) {
+func TestExecSpec_ResumeSkipsCompile(t *testing.T) {
 	tmp := t.TempDir()
 
 	// Create a prior run with tasks and worktree
@@ -175,6 +173,7 @@ func TestExecSpec_ResumeSkipsInitCompile(t *testing.T) {
 			&stageRecorder{name: "init", orderPtr: &order},
 			&stageRecorder{name: "compile", orderPtr: &order},
 			&stageRecorder{name: "plan", orderPtr: &order},
+			&stageRecorder{name: "write_contracts", orderPtr: &order},
 			&stageRecorder{name: "execute", orderPtr: &order},
 			&stageRecorder{name: "validate", orderPtr: &order},
 		},
@@ -193,8 +192,8 @@ func TestExecSpec_ResumeSkipsInitCompile(t *testing.T) {
 		t.Fatalf("unexpected error: %v", err)
 	}
 
-	// init and compile should be skipped
-	for _, skipped := range []string{"init", "compile"} {
+	// compile and init should be skipped (WorktreePath is set)
+	for _, skipped := range []string{"compile", "init"} {
 		for _, ran := range order {
 			if ran == skipped {
 				t.Errorf("stage %s should have been skipped on resume", skipped)
@@ -202,8 +201,8 @@ func TestExecSpec_ResumeSkipsInitCompile(t *testing.T) {
 		}
 	}
 
-	// plan, execute, validate should run (plan no-ops when tasks exist)
-	want := []string{"plan", "execute", "validate"}
+	// plan, write_contracts, execute, validate should run (init skipped because WorktreePath is set)
+	want := []string{"plan", "write_contracts", "execute", "validate"}
 	if len(order) != len(want) {
 		t.Fatalf("expected %d stages, got %d: %v", len(want), len(order), order)
 	}
@@ -283,18 +282,6 @@ func TestExecSpec_ResumeResetsGateFlags(t *testing.T) {
 	if capturedRS.TerminalReason != "" {
 		t.Errorf("TerminalReason should be empty, got %s", capturedRS.TerminalReason)
 	}
-	if capturedRS.Cycle != 3 {
-		// Prior cycle was 2, resume increments to 3, then SpecLoop sets it to cycle+1
-		// Actually SpecLoop sets rs.Cycle = cycle + 1 where cycle starts at 0,
-		// so SpecLoop overwrites it. Let's just check the pre-loop state.
-		// The stage captures it after SpecLoop sets Cycle = 1.
-		// So we need to verify the increment happened before SpecLoop.
-		// Actually, SpecLoop sets rs.Cycle = cycle + 1 at the top of the for loop.
-		// cycle starts at 0, so rs.Cycle = 1 after that assignment.
-		// Our resume set it to 3 but SpecLoop overwrites with 1.
-		// This is expected behavior - check that it was incremented before the loop,
-		// but SpecLoop controls it from there.
-	}
 	if !capturedRS.EndedAt.IsZero() {
 		t.Errorf("EndedAt should be zero, got %v", capturedRS.EndedAt)
 	}
diff --git a/cmd/gromit-next/stage_provider.go b/cmd/gromit-next/stage_provider.go
index f71030f5c..4881cfc58 100644
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
+		taskRunner     specloop.TaskRunner
+		finalVal       stages.FinalValidator
+		reviewRunner   stages.ReviewRunner
+		acceptEval     stages.AcceptEvaluator
+		contractWriter contract.ContractWriter
+		diffProv       review.DiffProvider = &noopDiffProvider{}
 	)
 
 	if p.claudeProvider != nil {
@@ -159,6 +161,14 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		acceptAgent := acceptor.NewProviderAcceptAgent(acceptAdapter)
 		acceptEval = acceptor.NewEvaluator(acceptAgent)
 
+		// Contract writer with FallbackAdapter (Sonnet/Planner tier).
+		contractAdapter := llmadapter.NewFallbackAdapter(
+			router, "contracts",
+			llmadapter.Config{Tier: policy.Models.Planner, OnCost: costCallback, OnInvocation: invocationCallback},
+			policy.Models.Planner,
+		)
+		contractWriter = contract.NewLLMContractWriter(contractAdapter)
+
 		diffProv = &lazyDiffProvider{rs: rs, fallbackDir: p.cfg.WorkDir}
 
 		// TODO: Wire real SpecCompilerAdapter here (blocked on ArtifactStore, cell resolution, level selection).
@@ -172,6 +182,7 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		finalVal = &noopValidator{}
 		reviewRunner = &noopReviewRunner{}
 		acceptEval = &noopAcceptEvaluator{}
+		contractWriter = &noopContractWriter{}
 	}
 
 	compileStage := stages.NewCompileStage(compiler, store, nil)
@@ -210,13 +221,22 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		EventLog:               eventLog,
 	})
 
-	validateStage := stages.NewValidateStage(finalVal, stages.ValidateStageConfig{
-		AlwaysRun: alwaysRun,
-		WorkDir:   p.cfg.WorkDir,
-	}, nil)
-
 	evidenceDir := store.RunEvidenceDir(rs.RunID)
 
+	contractEvaluator := &contract.DefaultContractEvaluator{}
+
+	writeContractsStage := stages.NewWriteContractsStage(contractWriter, stages.WriteContractsStageConfig{
+		SpecPath:    p.cfg.SpecPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}, budget, eventLog)
+
+	validateStage := stages.NewValidateStage(finalVal, stages.ValidateStageConfig{
+		AlwaysRun:   alwaysRun,
+		WorkDir:     p.cfg.WorkDir,
+		EvidenceDir: evidenceDir,
+	}, nil, contractEvaluator)
+
 	reviewStage := stages.NewReviewStage(reviewRunner, stages.ReviewStageConfig{
 		SpecContent:  string(specContent),
 		EvidenceDir:  evidenceDir,
@@ -247,6 +267,7 @@ func (p *RealStageProvider) BuildStages(policy execpolicy.Policy, rs *runstore.R
 		initStage,
 		compileStage,
 		planStage,
+		writeContractsStage,
 		executeStage,
 		validateStage,
 		reviewStage,
@@ -414,3 +435,10 @@ func (n *noopAcceptEvaluator) Evaluate(_ context.Context, _ acceptor.EvaluateInp
 	r.NormalizeNilFields()
 	return r, nil
 }
+
+// noopContractWriter satisfies contract.ContractWriter with a no-op.
+type noopContractWriter struct{}
+
+func (n *noopContractWriter) WriteContracts(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
+	return &contract.ScenarioContract{Scenarios: []contract.ScenarioAssertions{}}, nil
+}
diff --git a/cmd/gromit-next/stage_provider_test.go b/cmd/gromit-next/stage_provider_test.go
index 3191ae1d3..24e703c88 100644
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
+	// Verify 10 stages are returned (same as before — multi-provider doesn't change stage count)
+	if len(stages) != 10 {
+		t.Fatalf("expected 10 stages, got %d", len(stages))
 	}
 }
diff --git a/cmd/gromit/delegation_boundary_test.go b/cmd/gromit/delegation_boundary_test.go
index 563b8d346..2b389e62d 100644
--- a/cmd/gromit/delegation_boundary_test.go
+++ b/cmd/gromit/delegation_boundary_test.go
@@ -6,6 +6,7 @@ import (
 	"go/token"
 	"os"
 	"path/filepath"
+	"runtime"
 	"sort"
 	"strconv"
 	"strings"
@@ -160,20 +161,12 @@ func commandSourceFiles() ([]string, error) {
 }
 
 func commandSourceDir() (string, error) {
-	candidates := []string{"cmd/gromit", "."}
-	for _, candidate := range candidates {
-		info, err := os.Stat(candidate)
-		if err != nil || !info.IsDir() {
-			continue
-		}
-		if candidate == "." {
-			if _, err := os.Stat(filepath.Join(candidate, "cli_contract_test.go")); err != nil {
-				continue
-			}
-		}
-		return candidate, nil
+	_, file, _, ok := runtime.Caller(0)
+	if !ok {
+		return "", fmt.Errorf("failed to get caller info")
 	}
-	return "", fmt.Errorf("cmd/gromit directory not found")
+	// file is the path to delegation_boundary_test.go, which is in cmd/gromit
+	return filepath.Dir(file), nil
 }
 
 func sortedFilePaths(m map[string][]string) []string {
diff --git a/cmd/gromit/run_spec_flag_test.go b/cmd/gromit/run_spec_flag_test.go
index e4691e27b..0af70337b 100644
--- a/cmd/gromit/run_spec_flag_test.go
+++ b/cmd/gromit/run_spec_flag_test.go
@@ -109,6 +109,10 @@ func TestRunSpecTestEnvRestoresWorkingDirectory(t *testing.T) {
 	if err != nil {
 		t.Fatalf("getting working directory before setup: %v", err)
 	}
+	originalWD, err = filepath.EvalSymlinks(originalWD)
+	if err != nil {
+		t.Fatalf("resolving symlinks in working directory: %v", err)
+	}
 
 	_, cleanup := setupRunSpecTestEnv(t)
 	cleanup()
@@ -117,6 +121,10 @@ func TestRunSpecTestEnvRestoresWorkingDirectory(t *testing.T) {
 	if err != nil {
 		t.Fatalf("getting working directory after cleanup: %v", err)
 	}
+	currentWD, err = filepath.EvalSymlinks(currentWD)
+	if err != nil {
+		t.Fatalf("resolving symlinks in working directory after cleanup: %v", err)
+	}
 	if currentWD != originalWD {
 		t.Fatalf("expected working directory %q after cleanup, got %q", originalWD, currentWD)
 	}
diff --git a/internal/next/contract/evaluator.go b/internal/next/contract/evaluator.go
new file mode 100644
index 000000000..495b02357
--- /dev/null
+++ b/internal/next/contract/evaluator.go
@@ -0,0 +1,105 @@
+package contract
+
+import (
+	"context"
+	"errors"
+	"fmt"
+	"os"
+	"os/exec"
+	"path/filepath"
+	"strings"
+)
+
+// ContractEvaluator abstracts contract assertion evaluation for testability.
+type ContractEvaluator interface {
+	Evaluate(ctx context.Context, contract *ScenarioContract, workDir string) ([]ContractFailure, error)
+}
+
+// DefaultContractEvaluator is the default implementation of ContractEvaluator.
+type DefaultContractEvaluator struct{}
+
+// Evaluate checks every assertion in contract against workDir and returns all failures.
+// All assertions are checked — no short-circuit on first failure.
+// A nil contract returns empty failures.
+func (e *DefaultContractEvaluator) Evaluate(_ context.Context, contract *ScenarioContract, workDir string) ([]ContractFailure, error) {
+	if contract == nil {
+		return nil, nil
+	}
+	var failures []ContractFailure
+	for _, scenario := range contract.Scenarios {
+		for _, a := range scenario.Assertions {
+			if f := e.check(scenario.Name, a, workDir); f != nil {
+				failures = append(failures, *f)
+			}
+		}
+	}
+	return failures, nil
+}
+
+func (e *DefaultContractEvaluator) check(scenarioName string, a ContractAssertion, workDir string) *ContractFailure {
+	fail := func(assertionType, detail string) *ContractFailure {
+		return &ContractFailure{
+			ScenarioName:  scenarioName,
+			AssertionType: assertionType,
+			Details:       detail,
+		}
+	}
+
+	switch {
+	case a.FileExists != "":
+		path := filepath.Join(workDir, a.FileExists)
+		if _, err := os.Stat(path); err != nil {
+			return fail("file_exists", fmt.Sprintf("file %q does not exist", a.FileExists))
+		}
+
+	case a.FileNotExists != "":
+		path := filepath.Join(workDir, a.FileNotExists)
+		_, err := os.Stat(path)
+		if err == nil {
+			return fail("file_not_exists", fmt.Sprintf("file %q exists but should not", a.FileNotExists))
+		}
+		if !errors.Is(err, os.ErrNotExist) {
+			return fail("file_not_exists", fmt.Sprintf("cannot stat %q: %v", a.FileNotExists, err))
+		}
+
+	case a.FileContains != nil:
+		path := filepath.Join(workDir, a.FileContains.Path)
+		content, err := os.ReadFile(path)
+		if err != nil {
+			return fail("file_contains", fmt.Sprintf("cannot read %q: %v", a.FileContains.Path, err))
+		}
+		if !strings.Contains(string(content), a.FileContains.Pattern) {
+			return fail("file_contains", fmt.Sprintf("file %q does not contain %q", a.FileContains.Path, a.FileContains.Pattern))
+		}
+
+	case a.FileNotContains != nil:
+		path := filepath.Join(workDir, a.FileNotContains.Path)
+		content, err := os.ReadFile(path)
+		if err != nil {
+			// Nonexistent file trivially does not contain the pattern.
+			if errors.Is(err, os.ErrNotExist) {
+				return nil
+			}
+			return fail("file_not_contains", fmt.Sprintf("cannot read %q: %v", a.FileNotContains.Path, err))
+		}
+		if strings.Contains(string(content), a.FileNotContains.Pattern) {
+			return fail("file_not_contains", fmt.Sprintf("file %q contains %q but should not", a.FileNotContains.Path, a.FileNotContains.Pattern))
+		}
+
+	case a.FileNotModified != "":
+		cmd := exec.Command("git", "diff", "--name-only", "HEAD", "--", a.FileNotModified)
+		cmd.Dir = workDir
+		out, err := cmd.Output()
+		if err != nil {
+			return fail("file_not_modified", fmt.Sprintf("git diff failed for %q: %v", a.FileNotModified, err))
+		}
+		if strings.TrimSpace(string(out)) != "" {
+			return fail("file_not_modified", fmt.Sprintf("file %q has been modified", a.FileNotModified))
+		}
+
+	default:
+		return fail("invalid_assertion", "assertion has no fields set")
+	}
+
+	return nil
+}
diff --git a/internal/next/contract/evaluator_test.go b/internal/next/contract/evaluator_test.go
new file mode 100644
index 000000000..44c860995
--- /dev/null
+++ b/internal/next/contract/evaluator_test.go
@@ -0,0 +1,467 @@
+package contract
+
+import (
+	"context"
+	"os"
+	"os/exec"
+	"path/filepath"
+	"strings"
+	"testing"
+)
+
+// initGitRepo creates a git repo in dir, commits a file, and returns the dir.
+func initGitRepo(t *testing.T) string {
+	t.Helper()
+	dir := t.TempDir()
+	run := func(args ...string) {
+		t.Helper()
+		cmd := exec.Command(args[0], args[1:]...)
+		cmd.Dir = dir
+		out, err := cmd.CombinedOutput()
+		if err != nil {
+			t.Fatalf("command %v failed: %v\n%s", args, err, out)
+		}
+	}
+	run("git", "init")
+	run("git", "config", "user.email", "test@test.com")
+	run("git", "config", "user.name", "Test")
+	// Create an initial commit so HEAD exists.
+	run("git", "commit", "--allow-empty", "-m", "init")
+	return dir
+}
+
+func TestEvaluator_NilContract(t *testing.T) {
+	ev := &DefaultContractEvaluator{}
+	failures, err := ev.Evaluate(context.Background(), nil, t.TempDir())
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures for nil contract, got %v", failures)
+	}
+}
+
+func TestEvaluator_EmptyAssertions(t *testing.T) {
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{
+			{Name: "empty", Assertions: []ContractAssertion{}},
+		},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, t.TempDir())
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileExists_Pass(t *testing.T) {
+	dir := t.TempDir()
+	f := filepath.Join(dir, "foo.txt")
+	os.WriteFile(f, []byte("hello"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileExists: "foo.txt"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileExists_Fail(t *testing.T) {
+	dir := t.TempDir()
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileExists: "missing.txt"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_exists" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+	if failures[0].ScenarioName != "s" {
+		t.Errorf("wrong scenario name: %s", failures[0].ScenarioName)
+	}
+}
+
+func TestEvaluator_FileContains_Pass(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileContains: &FileContainsAssertion{Path: "a.txt", Pattern: "hello"}},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileContains_PatternMissing(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileContains: &FileContainsAssertion{Path: "a.txt", Pattern: "goodbye"}},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_contains" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+}
+
+func TestEvaluator_FileContains_NonexistentFile(t *testing.T) {
+	dir := t.TempDir()
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileContains: &FileContainsAssertion{Path: "no.txt", Pattern: "x"}},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure for nonexistent file, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_contains" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+}
+
+func TestEvaluator_FileNotExists_Pass(t *testing.T) {
+	dir := t.TempDir()
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotExists: "absent.txt"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileNotExists_Fail(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "present.txt"), []byte("x"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotExists: "present.txt"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_not_exists" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+}
+
+func TestEvaluator_FileNotExists_PermissionDenied(t *testing.T) {
+	dir := t.TempDir()
+	// Create a subdirectory and revoke read permissions
+	subdir := filepath.Join(dir, "restricted")
+	os.Mkdir(subdir, 0755)
+	os.Chmod(subdir, 0000)
+	defer os.Chmod(subdir, 0755) // Restore for cleanup
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotExists: "restricted/file.txt"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure for permission denied, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_not_exists" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+	// Check that the error message indicates "cannot stat" rather than "exists"
+	if !strings.Contains(failures[0].Details, "cannot stat") {
+		t.Errorf("expected 'cannot stat' in details, got: %s", failures[0].Details)
+	}
+}
+
+func TestEvaluator_FileNotContains_Pass(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotContains: &FileContainsAssertion{Path: "b.txt", Pattern: "goodbye"}},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileNotContains_Fail(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello world"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotContains: &FileContainsAssertion{Path: "b.txt", Pattern: "hello"}},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_not_contains" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+}
+
+func TestEvaluator_FileNotContains_NonexistentFile(t *testing.T) {
+	dir := t.TempDir()
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotContains: &FileContainsAssertion{Path: "no.txt", Pattern: "x"}},
+			},
+		}},
+	}
+	// A nonexistent file trivially does not contain the pattern — pass.
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures for nonexistent file in file_not_contains, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileNotModified_Pass(t *testing.T) {
+	dir := initGitRepo(t)
+	// Write a file and commit it — it is clean, not modified.
+	p := filepath.Join(dir, "clean.go")
+	os.WriteFile(p, []byte("package main"), 0644)
+	run := func(args ...string) {
+		cmd := exec.Command(args[0], args[1:]...)
+		cmd.Dir = dir
+		cmd.Run()
+	}
+	run("git", "add", "clean.go")
+	run("git", "commit", "-m", "add clean.go")
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotModified: "clean.go"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 0 {
+		t.Errorf("expected no failures for unmodified file, got %v", failures)
+	}
+}
+
+func TestEvaluator_FileNotModified_Fail(t *testing.T) {
+	dir := initGitRepo(t)
+	p := filepath.Join(dir, "dirty.go")
+	os.WriteFile(p, []byte("package main"), 0644)
+	run := func(args ...string) {
+		cmd := exec.Command(args[0], args[1:]...)
+		cmd.Dir = dir
+		cmd.Run()
+	}
+	run("git", "add", "dirty.go")
+	run("git", "commit", "-m", "add dirty.go")
+	// Modify the file after commit.
+	os.WriteFile(p, []byte("package main\n// changed"), 0644)
+	run("git", "add", "dirty.go")
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileNotModified: "dirty.go"},
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure for modified file, got %v", failures)
+	}
+	if failures[0].AssertionType != "file_not_modified" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+}
+
+func TestEvaluator_MultipleAssertions_PartialFailure_NoShortCircuit(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("content"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{FileExists: "exists.txt"},    // pass
+				{FileExists: "missing1.txt"},  // fail
+				{FileExists: "missing2.txt"},  // fail — must still be checked
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 2 {
+		t.Errorf("expected 2 failures (no short-circuit), got %d: %v", len(failures), failures)
+	}
+}
+
+func TestEvaluator_MultipleScenarios(t *testing.T) {
+	dir := t.TempDir()
+	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)
+
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{
+			{Name: "pass-scenario", Assertions: []ContractAssertion{
+				{FileExists: "a.txt"},
+			}},
+			{Name: "fail-scenario", Assertions: []ContractAssertion{
+				{FileExists: "b.txt"},
+			}},
+		},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure, got %v", failures)
+	}
+	if failures[0].ScenarioName != "fail-scenario" {
+		t.Errorf("wrong scenario name: %s", failures[0].ScenarioName)
+	}
+}
+
+func TestEvaluator_ZeroValueAssertion(t *testing.T) {
+	dir := t.TempDir()
+	ev := &DefaultContractEvaluator{}
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{{
+			Name: "s",
+			Assertions: []ContractAssertion{
+				{}, // zero-value assertion with no fields set
+			},
+		}},
+	}
+	failures, err := ev.Evaluate(context.Background(), &contract, dir)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(failures) != 1 {
+		t.Fatalf("expected 1 failure for zero-value assertion, got %d failures: %v", len(failures), failures)
+	}
+	if failures[0].AssertionType != "invalid_assertion" {
+		t.Errorf("wrong assertion type: %s", failures[0].AssertionType)
+	}
+	if !strings.Contains(failures[0].Details, "no fields set") {
+		t.Errorf("expected 'no fields set' in details, got: %s", failures[0].Details)
+	}
+}
diff --git a/internal/next/contract/llm_writer.go b/internal/next/contract/llm_writer.go
new file mode 100644
index 000000000..21ec9a3a4
--- /dev/null
+++ b/internal/next/contract/llm_writer.go
@@ -0,0 +1,52 @@
+package contract
+
+import (
+	"context"
+	"fmt"
+
+	"github.com/danabrams/gromit/internal/next/llmadapter"
+)
+
+// ContractWriter translates spec scenarios into declarative contract assertions.
+// The LLM-backed implementation receives spec scenarios and produces a ScenarioContract.
+type ContractWriter interface {
+	WriteContracts(ctx context.Context, scenarios []SpecScenario, specPacket string) (*ScenarioContract, error)
+}
+
+// LLMContractWriter implements ContractWriter using an LLM invoker.
+// It constructs a prompt from spec scenarios and packet, invokes the LLM,
+// and parses the YAML response into a ScenarioContract. Uses Sonnet (P1) model tier.
+type LLMContractWriter struct {
+	invoker llmadapter.Invoker
+}
+
+// NewLLMContractWriter creates an LLMContractWriter backed by the given invoker.
+func NewLLMContractWriter(invoker llmadapter.Invoker) *LLMContractWriter {
+	return &LLMContractWriter{invoker: invoker}
+}
+
+// WriteContracts implements ContractWriter, translating spec scenarios into
+// declarative contract assertions via LLM.
+func (w *LLMContractWriter) WriteContracts(ctx context.Context, scenarios []SpecScenario, specPacket string) (*ScenarioContract, error) {
+	prompt, err := RenderContractPrompt(ContractPromptInput{
+		SpecPacket: specPacket,
+		Scenarios:  scenarios,
+	})
+	if err != nil {
+		return nil, fmt.Errorf("render contract prompt: %w", err)
+	}
+
+	result, err := w.invoker.Invoke(ctx, prompt)
+	if err != nil {
+		return nil, err
+	}
+	if result == nil {
+		return nil, fmt.Errorf("contract writer: provider returned nil result")
+	}
+
+	c, err := ParseContractYAML(result.Output)
+	if err != nil {
+		return nil, err
+	}
+	return &c, nil
+}
diff --git a/internal/next/contract/llm_writer_test.go b/internal/next/contract/llm_writer_test.go
new file mode 100644
index 000000000..78f383e93
--- /dev/null
+++ b/internal/next/contract/llm_writer_test.go
@@ -0,0 +1,176 @@
+package contract
+
+import (
+	"context"
+	"errors"
+	"strings"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/provider"
+)
+
+// stubInvoker is a test double for llmadapter.Invoker.
+type stubInvoker struct {
+	output string
+	err    error
+}
+
+func (s *stubInvoker) Invoke(_ context.Context, _ string) (*provider.Result, error) {
+	if s.err != nil {
+		return nil, s.err
+	}
+	return &provider.Result{Output: s.output}, nil
+}
+
+func (s *stubInvoker) InvokeInDir(_ context.Context, _ string, _ string) (*provider.Result, error) {
+	return s.Invoke(context.Background(), "")
+}
+
+// nilResultInvoker returns nil result without error.
+type nilResultInvoker struct{}
+
+func (n *nilResultInvoker) Invoke(_ context.Context, _ string) (*provider.Result, error) {
+	return nil, nil
+}
+
+func (n *nilResultInvoker) InvokeInDir(_ context.Context, _ string, _ string) (*provider.Result, error) {
+	return nil, nil
+}
+
+// captureInvoker records the prompt passed to Invoke for assertion in tests.
+type captureInvoker struct {
+	capturedPrompt string
+	output         string
+}
+
+func (c *captureInvoker) Invoke(_ context.Context, prompt string) (*provider.Result, error) {
+	c.capturedPrompt = prompt
+	return &provider.Result{Output: c.output}, nil
+}
+
+func (c *captureInvoker) InvokeInDir(_ context.Context, prompt string, _ string) (*provider.Result, error) {
+	return c.Invoke(context.Background(), prompt)
+}
+
+// TestLLMContractWriter_Success verifies that WriteContracts returns a non-nil
+// *ScenarioContract pointer with correct data on success.
+func TestLLMContractWriter_Success(t *testing.T) {
+	yamlOutput := `scenarios:
+  - name: add-works
+    assertions:
+      - file_exists: calc/calc.go`
+	inv := &stubInvoker{output: yamlOutput}
+	w := NewLLMContractWriter(inv)
+
+	scenarios := []SpecScenario{
+		{Name: "add-works", When: "add is called", Then: "result is 3"},
+	}
+	result, err := w.WriteContracts(context.Background(), scenarios, "spec packet")
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if result == nil {
+		t.Fatal("expected non-nil *ScenarioContract")
+	}
+	if len(result.Scenarios) != 1 {
+		t.Fatalf("expected 1 scenario, got %d", len(result.Scenarios))
+	}
+	if result.Scenarios[0].Name != "add-works" {
+		t.Fatalf("expected scenario name 'add-works', got %q", result.Scenarios[0].Name)
+	}
+}
+
+// TestLLMContractWriter_InvokerError verifies that WriteContracts returns nil pointer
+// and an error when the invoker fails.
+func TestLLMContractWriter_InvokerError(t *testing.T) {
+	inv := &stubInvoker{err: errors.New("llm unavailable")}
+	w := NewLLMContractWriter(inv)
+
+	result, err := w.WriteContracts(context.Background(), []SpecScenario{}, "packet")
+	if err == nil {
+		t.Fatal("expected error when invoker fails")
+	}
+	if result != nil {
+		t.Fatalf("expected nil pointer on error, got %+v", result)
+	}
+}
+
+// TestLLMContractWriter_NilResult verifies that WriteContracts returns nil pointer
+// and an error when the invoker returns a nil result without error.
+func TestLLMContractWriter_NilResult(t *testing.T) {
+	w := NewLLMContractWriter(&nilResultInvoker{})
+
+	result, err := w.WriteContracts(context.Background(), []SpecScenario{}, "packet")
+	if err == nil {
+		t.Fatal("expected error for nil invoker result")
+	}
+	if result != nil {
+		t.Fatalf("expected nil pointer on nil invoker result, got %+v", result)
+	}
+}
+
+// TestLLMContractWriter_YAMLFence verifies that YAML fences in LLM output are stripped.
+func TestLLMContractWriter_YAMLFence(t *testing.T) {
+	yamlOutput := "Here you go:\n```yaml\nscenarios:\n  - name: test\n    assertions:\n      - file_exists: main.go\n```\n"
+	inv := &stubInvoker{output: yamlOutput}
+	w := NewLLMContractWriter(inv)
+
+	scenarios := []SpecScenario{
+		{Name: "test", Then: "main.go exists"},
+	}
+	result, err := w.WriteContracts(context.Background(), scenarios, "packet")
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if result == nil {
+		t.Fatal("expected non-nil *ScenarioContract")
+	}
+	if len(result.Scenarios) != 1 || result.Scenarios[0].Name != "test" {
+		t.Fatalf("expected 1 scenario named 'test', got %v", result.Scenarios)
+	}
+}
+
+// TestLLMContractWriter_SpecPacketIncludedInPrompt verifies that specPacket
+// content is included in the rendered prompt.
+func TestLLMContractWriter_SpecPacketIncludedInPrompt(t *testing.T) {
+	yamlOutput := `scenarios:
+  - name: add-works
+    assertions:
+      - file_exists: calc/calc.go`
+	inv := &captureInvoker{output: yamlOutput}
+	w := NewLLMContractWriter(inv)
+
+	specPacket := "# Prior Validation Errors\nfile_exists: calc/calc.go failed"
+	scenarios := []SpecScenario{
+		{Name: "add-works", When: "add is called", Then: "result is 3"},
+	}
+	_, err := w.WriteContracts(context.Background(), scenarios, specPacket)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if !strings.Contains(inv.capturedPrompt, specPacket) {
+		t.Errorf("expected prompt to contain specPacket;\ngot: %q", inv.capturedPrompt)
+	}
+}
+
+// TestLLMContractWriter_SameInputsSamePrompt verifies that the same inputs
+// produce the same prompt.
+func TestLLMContractWriter_SameInputsSamePrompt(t *testing.T) {
+	yamlOutput := `scenarios:
+  - name: add-works
+    assertions:
+      - file_exists: calc/calc.go`
+	inv1 := &captureInvoker{output: yamlOutput}
+	inv2 := &captureInvoker{output: yamlOutput}
+	w1 := NewLLMContractWriter(inv1)
+	w2 := NewLLMContractWriter(inv2)
+
+	scenarios := []SpecScenario{{Name: "add-works", When: "add is called", Then: "result is 3"}}
+	specPacket := "spec packet"
+	_, _ = w1.WriteContracts(context.Background(), scenarios, specPacket)
+	_, _ = w2.WriteContracts(context.Background(), scenarios, specPacket)
+
+	if inv1.capturedPrompt != inv2.capturedPrompt {
+		t.Error("same inputs should produce the same prompt")
+	}
+}
diff --git a/internal/next/contract/parse.go b/internal/next/contract/parse.go
new file mode 100644
index 000000000..98f426799
--- /dev/null
+++ b/internal/next/contract/parse.go
@@ -0,0 +1,132 @@
+package contract
+
+import (
+	"fmt"
+	"strings"
+)
+
+// ParseScenarios extracts scenarios from spec markdown by matching "### Scenario:" headers
+// and parsing Given/When/Then/Notes blocks. Scenarios missing When or Then are skipped;
+// their names with reasons are returned as the second return value. Given and Notes are
+// optional. Returns empty slices (never nil) when no Scenarios section is found or it is empty.
+func ParseScenarios(specMarkdown string) ([]SpecScenario, []string, error) {
+	lines := strings.Split(specMarkdown, "\n")
+
+	inScenariosSection := false
+	var scenarios []SpecScenario
+	var skipped []string
+
+	// current scenario being built
+	var cur *SpecScenario
+	currentBlock := "" // which block we're currently collecting: "given","when","then","notes"
+	var blockLines []string
+
+	flushBlock := func() {
+		if cur == nil || currentBlock == "" {
+			return
+		}
+		text := strings.TrimSpace(strings.Join(blockLines, "\n"))
+		switch currentBlock {
+		case "given":
+			cur.Given = text
+		case "when":
+			cur.When = text
+		case "then":
+			cur.Then = text
+		case "notes":
+			cur.Notes = text
+		}
+		currentBlock = ""
+		blockLines = nil
+	}
+
+	flushScenario := func() {
+		if cur == nil {
+			return
+		}
+		flushBlock()
+		if cur.When == "" || cur.Then == "" {
+			skipped = append(skipped, fmt.Sprintf("%s: missing When or Then block", cur.Name))
+		} else {
+			scenarios = append(scenarios, *cur)
+		}
+		cur = nil
+	}
+
+	for _, line := range lines {
+		trimmed := strings.TrimSpace(line)
+
+		// Detect ## Scenarios section header (exact match)
+		if strings.HasPrefix(trimmed, "## ") {
+			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
+			if heading == "Scenarios" {
+				inScenariosSection = true
+				continue
+			}
+			// Any other ## heading ends the Scenarios section
+			if inScenariosSection {
+				flushScenario()
+				inScenariosSection = false
+			}
+			continue
+		}
+
+		if !inScenariosSection {
+			continue
+		}
+
+		// Detect ### Scenario: header
+		if strings.HasPrefix(trimmed, "### Scenario:") {
+			flushScenario()
+			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "### Scenario:"))
+			cur = &SpecScenario{Name: name}
+			currentBlock = ""
+			blockLines = nil
+			continue
+		}
+
+		if cur == nil {
+			continue
+		}
+
+		// Detect block markers: **Given:**, **When:**, **Then:**, **Notes:**
+		if block, rest, ok := parseBlockMarker(trimmed); ok {
+			flushBlock()
+			currentBlock = block
+			blockLines = nil
+			if rest != "" {
+				blockLines = append(blockLines, rest)
+			}
+			continue
+		}
+
+		// Accumulate into current block
+		if currentBlock != "" {
+			blockLines = append(blockLines, line)
+		}
+	}
+
+	// Flush last scenario
+	flushScenario()
+
+	if scenarios == nil {
+		scenarios = []SpecScenario{}
+	}
+	if skipped == nil {
+		skipped = []string{}
+	}
+	return scenarios, skipped, nil
+}
+
+// parseBlockMarker checks if the line is a **Given:**, **When:**, **Then:**, or **Notes:** marker.
+// Returns the block name (lowercased), the rest of the text on the same line, and true if matched.
+func parseBlockMarker(line string) (block, rest string, ok bool) {
+	for _, name := range []string{"Given", "When", "Then", "Notes"} {
+		prefix := "**" + name + ":**"
+		if strings.HasPrefix(line, prefix) {
+			rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
+			return strings.ToLower(name), rest, true
+		}
+	}
+	return "", "", false
+}
diff --git a/internal/next/contract/parse_test.go b/internal/next/contract/parse_test.go
new file mode 100644
index 000000000..84caf9c05
--- /dev/null
+++ b/internal/next/contract/parse_test.go
@@ -0,0 +1,288 @@
+package contract
+
+import (
+	"strings"
+	"testing"
+)
+
+func TestParseContractYAML_ValidYAML(t *testing.T) {
+	input := `scenarios:
+  - name: add-works
+    assertions:
+      - file_exists: calc/calc.go`
+	c, err := ParseContractYAML(input)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(c.Scenarios) != 1 {
+		t.Fatalf("expected 1 scenario, got %d", len(c.Scenarios))
+	}
+	if c.Scenarios[0].Name != "add-works" {
+		t.Fatalf("expected scenario name 'add-works', got %q", c.Scenarios[0].Name)
+	}
+	if len(c.Scenarios[0].Assertions) != 1 {
+		t.Fatalf("expected 1 assertion, got %d", len(c.Scenarios[0].Assertions))
+	}
+	if c.Scenarios[0].Assertions[0].FileExists != "calc/calc.go" {
+		t.Fatalf("expected FileExists='calc/calc.go', got %q", c.Scenarios[0].Assertions[0].FileExists)
+	}
+}
+
+func TestParseContractYAML_StripsYAMLFence(t *testing.T) {
+	output := "Here is the YAML:\n```yaml\nscenarios:\n  - name: test\n    assertions:\n      - file_exists: foo.go\n```\n"
+	c, err := ParseContractYAML(output)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(c.Scenarios) != 1 || c.Scenarios[0].Name != "test" {
+		t.Fatalf("expected 1 scenario named 'test', got %v", c.Scenarios)
+	}
+}
+
+func TestParseContractYAML_StripsGenericFence(t *testing.T) {
+	output := "```\nscenarios:\n  - name: sub-works\n    assertions:\n      - file_not_exists: old.go\n```"
+	c, err := ParseContractYAML(output)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(c.Scenarios) != 1 || c.Scenarios[0].Name != "sub-works" {
+		t.Fatalf("expected 1 scenario named 'sub-works', got %v", c.Scenarios)
+	}
+}
+
+func TestParseContractYAML_AllAssertionTypes(t *testing.T) {
+	input := `scenarios:
+  - name: all-types
+    assertions:
+      - file_exists: a.go
+      - file_not_exists: b.go
+      - file_not_modified: c.go
+      - file_contains:
+          path: d.go
+          pattern: hello
+      - file_not_contains:
+          path: e.go
+          pattern: world`
+	c, err := ParseContractYAML(input)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if len(c.Scenarios) != 1 {
+		t.Fatalf("expected 1 scenario, got %d", len(c.Scenarios))
+	}
+	assertions := c.Scenarios[0].Assertions
+	if len(assertions) != 5 {
+		t.Fatalf("expected 5 assertions, got %d", len(assertions))
+	}
+	if assertions[0].FileExists != "a.go" {
+		t.Errorf("assertion[0]: expected FileExists='a.go', got %q", assertions[0].FileExists)
+	}
+	if assertions[1].FileNotExists != "b.go" {
+		t.Errorf("assertion[1]: expected FileNotExists='b.go', got %q", assertions[1].FileNotExists)
+	}
+	if assertions[2].FileNotModified != "c.go" {
+		t.Errorf("assertion[2]: expected FileNotModified='c.go', got %q", assertions[2].FileNotModified)
+	}
+	if assertions[3].FileContains == nil || assertions[3].FileContains.Path != "d.go" || assertions[3].FileContains.Pattern != "hello" {
+		t.Errorf("assertion[3]: unexpected FileContains: %+v", assertions[3].FileContains)
+	}
+	if assertions[4].FileNotContains == nil || assertions[4].FileNotContains.Path != "e.go" || assertions[4].FileNotContains.Pattern != "world" {
+		t.Errorf("assertion[4]: unexpected FileNotContains: %+v", assertions[4].FileNotContains)
+	}
+}
+
+func TestParseContractYAML_InvalidYAML(t *testing.T) {
+	_, err := ParseContractYAML("not: [valid: yaml:")
+	if err == nil {
+		t.Fatal("expected error for invalid YAML")
+	}
+}
+
+func TestParseScenarios_MultipleScenarios(t *testing.T) {
+	specMD := `# My Spec
+
+## Scenarios
+
+### Scenario: Add function works
+
+**Given:** A calculator repo
+**When:** The pipeline executes
+**Then:**
+- The Add function is implemented
+- calc.go exists
+
+### Scenario: Subtract function works
+
+**Given:** A calculator repo
+**When:** The pipeline executes
+**Then:**
+- The Subtract function is implemented
+`
+	scenarios, skipped, err := ParseScenarios(specMD)
+	if err != nil {
+		t.Fatalf("ParseScenarios: %v", err)
+	}
+	if len(skipped) != 0 {
+		t.Errorf("expected no skipped scenarios, got %v", skipped)
+	}
+	if len(scenarios) != 2 {
+		t.Fatalf("expected 2 scenarios, got %d: %v", len(scenarios), scenarios)
+	}
+	if scenarios[0].Name != "Add function works" {
+		t.Errorf("scenarios[0].Name = %q, want %q", scenarios[0].Name, "Add function works")
+	}
+	if scenarios[1].Name != "Subtract function works" {
+		t.Errorf("scenarios[1].Name = %q, want %q", scenarios[1].Name, "Subtract function works")
+	}
+	if !strings.Contains(scenarios[0].When, "pipeline executes") {
+		t.Errorf("scenarios[0].When = %q, expected to contain 'pipeline executes'", scenarios[0].When)
+	}
+	if !strings.Contains(scenarios[0].Then, "Add function is implemented") {
+		t.Errorf("scenarios[0].Then = %q, expected to contain 'Add function is implemented'", scenarios[0].Then)
+	}
+	if !strings.Contains(scenarios[0].Given, "calculator repo") {
+		t.Errorf("scenarios[0].Given = %q, expected to contain 'calculator repo'", scenarios[0].Given)
+	}
+}
+
+func TestParseScenarios_MissingWhenSkipped(t *testing.T) {
+	specMD := `# My Spec
+
+## Scenarios
+
+### Scenario: Has when and then
+
+**When:** Something happens
+**Then:** Something is true
+
+### Scenario: Missing when
+
+**Given:** Some context
+**Then:** Something is true
+
+### Scenario: Also valid
+
+**When:** Another thing
+**Then:** Another result
+`
+	scenarios, skipped, err := ParseScenarios(specMD)
+	if err != nil {
+		t.Fatalf("ParseScenarios: %v", err)
+	}
+	if len(scenarios) != 2 {
+		t.Fatalf("expected 2 scenarios (one skipped for missing When), got %d: %v", len(scenarios), scenarios)
+	}
+	if scenarios[0].Name != "Has when and then" {
+		t.Errorf("scenarios[0].Name = %q", scenarios[0].Name)
+	}
+	if scenarios[1].Name != "Also valid" {
+		t.Errorf("scenarios[1].Name = %q", scenarios[1].Name)
+	}
+	if len(skipped) != 1 {
+		t.Fatalf("expected 1 skipped scenario, got %d: %v", len(skipped), skipped)
+	}
+	if !strings.Contains(skipped[0], "Missing when") {
+		t.Errorf("skipped[0] = %q, expected to contain 'Missing when'", skipped[0])
+	}
+}
+
+func TestParseScenarios_MissingThenSkipped(t *testing.T) {
+	specMD := `# My Spec
+
+## Scenarios
+
+### Scenario: Has when and then
+
+**When:** Something happens
+**Then:** Something is true
+
+### Scenario: Missing then
+
+**Given:** Some context
+**When:** Something happens
+`
+	scenarios, skipped, err := ParseScenarios(specMD)
+	if err != nil {
+		t.Fatalf("ParseScenarios: %v", err)
+	}
+	if len(scenarios) != 1 {
+		t.Fatalf("expected 1 scenario (one skipped for missing Then), got %d: %v", len(scenarios), scenarios)
+	}
+	if scenarios[0].Name != "Has when and then" {
+		t.Errorf("scenarios[0].Name = %q", scenarios[0].Name)
+	}
+	if len(skipped) != 1 {
+		t.Fatalf("expected 1 skipped scenario, got %d: %v", len(skipped), skipped)
+	}
+	if !strings.Contains(skipped[0], "Missing then") {
+		t.Errorf("skipped[0] = %q, expected to contain 'Missing then'", skipped[0])
+	}
+}
+
+func TestParseScenarios_OptionalGivenAndNotes(t *testing.T) {
+	specMD := `# My Spec
+
+## Scenarios
+
+### Scenario: No given no notes
+
+**When:** Something happens
+**Then:** Something is true
+
+### Scenario: Has notes
+
+**When:** Another thing
+**Then:** Another result
+**Notes:** This is a note about the scenario
+`
+	scenarios, _, err := ParseScenarios(specMD)
+	if err != nil {
+		t.Fatalf("ParseScenarios: %v", err)
+	}
+	if len(scenarios) != 2 {
+		t.Fatalf("expected 2 scenarios, got %d", len(scenarios))
+	}
+	if scenarios[0].Given != "" {
+		t.Errorf("scenarios[0].Given should be empty, got %q", scenarios[0].Given)
+	}
+	if scenarios[0].Notes != "" {
+		t.Errorf("scenarios[0].Notes should be empty, got %q", scenarios[0].Notes)
+	}
+	if !strings.Contains(scenarios[1].Notes, "note about the scenario") {
+		t.Errorf("scenarios[1].Notes = %q, expected to contain 'note about the scenario'", scenarios[1].Notes)
+	}
+}
+
+func TestParseScenarios_EmptyScenariosSection(t *testing.T) {
+	specMD := `# My Spec
+
+## Scenarios
+
+## Next Section
+`
+	scenarios, _, err := ParseScenarios(specMD)
+	if err != nil {
+		t.Fatalf("ParseScenarios: %v", err)
+	}
+	if len(scenarios) != 0 {
+		t.Errorf("expected 0 scenarios for empty section, got %d", len(scenarios))
+	}
+}
+
+func TestParseScenarios_NoScenariosSection(t *testing.T) {
+	specMD := `# My Spec
+
+## Description
+Some description here.
+
+## Acceptance Criteria
+- Something
+`
+	scenarios, _, err := ParseScenarios(specMD)
+	if err != nil {
+		t.Fatalf("ParseScenarios: %v", err)
+	}
+	if len(scenarios) != 0 {
+		t.Errorf("expected 0 scenarios when no Scenarios section, got %d", len(scenarios))
+	}
+}
diff --git a/internal/next/contract/prompt.go b/internal/next/contract/prompt.go
new file mode 100644
index 000000000..453112403
--- /dev/null
+++ b/internal/next/contract/prompt.go
@@ -0,0 +1,28 @@
+package contract
+
+import (
+	"bytes"
+	_ "embed"
+	"text/template"
+)
+
+//go:embed prompt.txt
+var contractPromptText string
+
+// ContractPromptInput holds the context for rendering a contract-writing prompt.
+type ContractPromptInput struct {
+	SpecPacket string
+	Scenarios  []SpecScenario
+}
+
+var contractPromptTmpl = template.Must(template.New("contract").Parse(contractPromptText))
+
+// RenderContractPrompt renders a prompt instructing the LLM to translate
+// Given/When/Then scenarios into YAML contract assertions.
+func RenderContractPrompt(input ContractPromptInput) (string, error) {
+	var buf bytes.Buffer
+	if err := contractPromptTmpl.Execute(&buf, input); err != nil {
+		return "", err
+	}
+	return buf.String(), nil
+}
diff --git a/internal/next/contract/prompt.txt b/internal/next/contract/prompt.txt
new file mode 100644
index 000000000..610812573
--- /dev/null
+++ b/internal/next/contract/prompt.txt
@@ -0,0 +1,71 @@
+You are translating Given/When/Then acceptance scenarios into declarative filesystem assertions.
+
+{{- if .SpecPacket}}
+
+## Spec
+
+{{.SpecPacket}}
+{{- end}}
+
+## Scenarios to translate
+
+{{range .Scenarios -}}
+### Scenario: {{.Name}}
+{{- if .Given}}
+**Given:** {{.Given}}
+{{- end}}
+{{- if .When}}
+**When:** {{.When}}
+{{- end}}
+{{- if .Then}}
+**Then:** {{.Then}}
+{{- end}}
+{{- if .Notes}}
+**Notes:** {{.Notes}}
+{{- end}}
+
+{{end}}
+## Instructions
+
+Translate each scenario's Then clause (and Given/When context) into concrete filesystem assertions.
+Use only these five assertion types — one per assertion object:
+
+1. **file_exists**: A file must exist.
+   `file_exists: path/to/file`
+
+2. **file_not_exists**: A file must not exist.
+   `file_not_exists: path/to/file`
+
+3. **file_contains**: A file must contain a literal substring.
+   `file_contains:`
+   `  path: path/to/file`
+   `  pattern: literal substring`
+
+4. **file_not_contains**: A file must NOT contain a literal substring.
+   `file_not_contains:`
+   `  path: path/to/file`
+   `  pattern: literal substring`
+
+5. **file_not_modified**: A file must be unmodified relative to HEAD (git diff must be empty).
+   `file_not_modified: path/to/file`
+
+Rules:
+- Each assertion object must have EXACTLY ONE field set.
+- Use relative paths from the project root.
+- Prefer specific assertions (file_contains) over general ones (file_exists) where the Then clause specifies content.
+- Do not invent assertions not derivable from the Given/When/Then text.
+
+Respond with ONLY a YAML block in this exact structure:
+
+```yaml
+scenarios:
+  - name: <scenario-name>
+    assertions:
+      - file_exists: path/to/file
+      - file_contains:
+          path: path/to/file
+          pattern: expected content
+  - name: <another-scenario>
+    assertions:
+      - file_exists: path/to/another/file
+```
diff --git a/internal/next/contract/types.go b/internal/next/contract/types.go
new file mode 100644
index 000000000..c2da78c87
--- /dev/null
+++ b/internal/next/contract/types.go
@@ -0,0 +1,46 @@
+package contract
+
+// SpecScenario represents a single scenario parsed from the spec's Scenarios section.
+// Extracted from spec markdown by matching "### Scenario:" headers and Given/When/Then blocks.
+type SpecScenario struct {
+	Name  string // Scenario title (from ### header, minus "Scenario: " prefix)
+	Given string // Given block text
+	When  string // When block text
+	Then  string // Then block text
+	Notes string // Notes block text (optional)
+}
+
+// ScenarioContract is the contract assertion file written by the WriteContracts stage.
+type ScenarioContract struct {
+	Scenarios []ScenarioAssertions `yaml:"scenarios"`
+}
+
+// ScenarioAssertions holds the assertions for a single scenario.
+type ScenarioAssertions struct {
+	Name       string              `yaml:"name"`
+	Assertions []ContractAssertion `yaml:"assertions"`
+}
+
+// ContractAssertion is a typed subset of e2e.Assertion, only filesystem-checkable fields.
+// This is a separate type from e2e.Assertion; do not import the e2e package.
+// Single-key map — exactly one field must be set per assertion.
+type ContractAssertion struct {
+	FileExists      string                 `yaml:"file_exists,omitempty"`
+	FileContains    *FileContainsAssertion `yaml:"file_contains,omitempty"`
+	FileNotModified string                 `yaml:"file_not_modified,omitempty"`
+	FileNotExists   string                 `yaml:"file_not_exists,omitempty"`
+	FileNotContains *FileContainsAssertion `yaml:"file_not_contains,omitempty"`
+}
+
+// FileContainsAssertion holds path and pattern for file_contains / file_not_contains assertions.
+type FileContainsAssertion struct {
+	Path    string `yaml:"path"`
+	Pattern string `yaml:"pattern"` // Literal substring, matched via strings.Contains
+}
+
+// ContractFailure represents a single failed contract assertion.
+type ContractFailure struct {
+	ScenarioName  string // e.g., "subtract-works"
+	AssertionType string // e.g., "file_contains"
+	Details       string // Human-readable failure description
+}
diff --git a/internal/next/contract/types_test.go b/internal/next/contract/types_test.go
new file mode 100644
index 000000000..034ebad72
--- /dev/null
+++ b/internal/next/contract/types_test.go
@@ -0,0 +1,138 @@
+package contract
+
+import (
+	"testing"
+
+	"gopkg.in/yaml.v3"
+)
+
+func TestSpecScenario(t *testing.T) {
+	s := SpecScenario{
+		Name:  "add-works",
+		Given: "a calculator",
+		When:  "I add 1 and 2",
+		Then:  "the result is 3",
+		Notes: "basic smoke test",
+	}
+	if s.Name != "add-works" {
+		t.Errorf("Name = %q, want %q", s.Name, "add-works")
+	}
+	if s.Notes != "basic smoke test" {
+		t.Errorf("Notes = %q, want %q", s.Notes, "basic smoke test")
+	}
+}
+
+func TestScenarioContract(t *testing.T) {
+	contract := ScenarioContract{
+		Scenarios: []ScenarioAssertions{
+			{
+				Name: "add-works",
+				Assertions: []ContractAssertion{
+					{FileExists: "output.txt"},
+				},
+			},
+		},
+	}
+
+	data, err := yaml.Marshal(&contract)
+	if err != nil {
+		t.Fatalf("yaml.Marshal: %v", err)
+	}
+
+	var got ScenarioContract
+	if err := yaml.Unmarshal(data, &got); err != nil {
+		t.Fatalf("yaml.Unmarshal: %v", err)
+	}
+
+	if len(got.Scenarios) != 1 {
+		t.Fatalf("len(Scenarios) = %d, want 1", len(got.Scenarios))
+	}
+	if got.Scenarios[0].Name != "add-works" {
+		t.Errorf("Name = %q, want %q", got.Scenarios[0].Name, "add-works")
+	}
+	if len(got.Scenarios[0].Assertions) != 1 {
+		t.Fatalf("len(Assertions) = %d, want 1", len(got.Scenarios[0].Assertions))
+	}
+	if got.Scenarios[0].Assertions[0].FileExists != "output.txt" {
+		t.Errorf("FileExists = %q, want %q", got.Scenarios[0].Assertions[0].FileExists, "output.txt")
+	}
+}
+
+func TestContractAssertion(t *testing.T) {
+	tests := []struct {
+		name      string
+		assertion ContractAssertion
+		wantKey   string
+	}{
+		{
+			name:      "file_exists",
+			assertion: ContractAssertion{FileExists: "foo.go"},
+			wantKey:   "file_exists",
+		},
+		{
+			name: "file_contains",
+			assertion: ContractAssertion{FileContains: &FileContainsAssertion{
+				Path:    "foo.go",
+				Pattern: "hello",
+			}},
+			wantKey: "file_contains",
+		},
+		{
+			name:      "file_not_modified",
+			assertion: ContractAssertion{FileNotModified: "bar.go"},
+			wantKey:   "file_not_modified",
+		},
+		{
+			name:      "file_not_exists",
+			assertion: ContractAssertion{FileNotExists: "baz.go"},
+			wantKey:   "file_not_exists",
+		},
+		{
+			name: "file_not_contains",
+			assertion: ContractAssertion{FileNotContains: &FileContainsAssertion{
+				Path:    "baz.go",
+				Pattern: "secret",
+			}},
+			wantKey: "file_not_contains",
+		},
+	}
+
+	for _, tc := range tests {
+		t.Run(tc.name, func(t *testing.T) {
+			data, err := yaml.Marshal(&tc.assertion)
+			if err != nil {
+				t.Fatalf("yaml.Marshal: %v", err)
+			}
+			if len(data) == 0 {
+				t.Fatal("expected non-empty YAML output")
+			}
+
+			var got ContractAssertion
+			if err := yaml.Unmarshal(data, &got); err != nil {
+				t.Fatalf("yaml.Unmarshal: %v", err)
+			}
+			// Re-marshal to compare
+			data2, err := yaml.Marshal(&got)
+			if err != nil {
+				t.Fatalf("yaml.Marshal round-trip: %v", err)
+			}
+			if string(data) != string(data2) {
+				t.Errorf("round-trip mismatch:\n  got:  %s\n  want: %s", data2, data)
+			}
+		})
+	}
+}
+
+func TestContractFailure(t *testing.T) {
+	f := ContractFailure{
+		ScenarioName:  "subtract-works",
+		AssertionType: "file_contains",
+		Details:       "expected 'result: -1' in output.txt",
+	}
+	if f.ScenarioName != "subtract-works" {
+		t.Errorf("ScenarioName = %q, want %q", f.ScenarioName, "subtract-works")
+	}
+	if f.AssertionType != "file_contains" {
+		t.Errorf("AssertionType = %q, want %q", f.AssertionType, "file_contains")
+	}
+}
diff --git a/internal/next/contract/validate.go b/internal/next/contract/validate.go
new file mode 100644
index 000000000..ec5ca7171
--- /dev/null
+++ b/internal/next/contract/validate.go
@@ -0,0 +1,68 @@
+package contract
+
+import (
+	"fmt"
+	"strings"
+
+	"gopkg.in/yaml.v3"
+)
+
+// ValidateContract checks each ContractAssertion in the contract against the known
+// assertion vocabulary. Each assertion must have exactly one field set; zero fields
+// or multiple fields are vocabulary violations. Returns a slice of error strings
+// describing any violations found.
+func ValidateContract(c ScenarioContract) []string {
+	var errs []string
+	for _, scenario := range c.Scenarios {
+		for i, a := range scenario.Assertions {
+			count := 0
+			if a.FileExists != "" {
+				count++
+			}
+			if a.FileContains != nil {
+				count++
+			}
+			if a.FileNotModified != "" {
+				count++
+			}
+			if a.FileNotExists != "" {
+				count++
+			}
+			if a.FileNotContains != nil {
+				count++
+			}
+			if count != 1 {
+				errs = append(errs, fmt.Sprintf("scenario %q assertion %d: expected exactly 1 field set, got %d", scenario.Name, i, count))
+			}
+		}
+	}
+	return errs
+}
+
+// ParseContractYAML extracts YAML from raw LLM output and unmarshals it into a ScenarioContract.
+// It strips markdown YAML fences and plain code fences before parsing.
+func ParseContractYAML(output string) (ScenarioContract, error) {
+	extracted := extractYAML(output)
+	var c ScenarioContract
+	if err := yaml.Unmarshal([]byte(extracted), &c); err != nil {
+		return ScenarioContract{}, fmt.Errorf("parse contract YAML: %w", err)
+	}
+	return c, nil
+}
+
+// extractYAML strips YAML code fences from LLM output.
+func extractYAML(output string) string {
+	if idx := strings.Index(output, "```yaml"); idx >= 0 {
+		start := idx + len("```yaml")
+		if end := strings.Index(output[start:], "```"); end >= 0 {
+			return strings.TrimSpace(output[start : start+end])
+		}
+	}
+	if idx := strings.Index(output, "```"); idx >= 0 {
+		start := idx + len("```")
+		if end := strings.Index(output[start:], "```"); end >= 0 {
+			return strings.TrimSpace(output[start : start+end])
+		}
+	}
+	return strings.TrimSpace(output)
+}
diff --git a/internal/next/contract/validate_test.go b/internal/next/contract/validate_test.go
new file mode 100644
index 000000000..c99229c80
--- /dev/null
+++ b/internal/next/contract/validate_test.go
@@ -0,0 +1,109 @@
+package contract
+
+import (
+	"testing"
+)
+
+func TestValidateContract_ValidSingleField(t *testing.T) {
+	tests := []struct {
+		name      string
+		assertion ContractAssertion
+	}{
+		{
+			name:      "file_exists",
+			assertion: ContractAssertion{FileExists: "some/file.go"},
+		},
+		{
+			name: "file_contains",
+			assertion: ContractAssertion{FileContains: &FileContainsAssertion{
+				Path:    "some/file.go",
+				Pattern: "hello",
+			}},
+		},
+		{
+			name:      "file_not_modified",
+			assertion: ContractAssertion{FileNotModified: "some/file.go"},
+		},
+		{
+			name:      "file_not_exists",
+			assertion: ContractAssertion{FileNotExists: "some/file.go"},
+		},
+		{
+			name: "file_not_contains",
+			assertion: ContractAssertion{FileNotContains: &FileContainsAssertion{
+				Path:    "some/file.go",
+				Pattern: "hello",
+			}},
+		},
+	}
+	for _, tt := range tests {
+		t.Run(tt.name, func(t *testing.T) {
+			c := ScenarioContract{
+				Scenarios: []ScenarioAssertions{
+					{Name: "s1", Assertions: []ContractAssertion{tt.assertion}},
+				},
+			}
+			errs := ValidateContract(c)
+			if len(errs) != 0 {
+				t.Errorf("expected no errors, got %v", errs)
+			}
+		})
+	}
+}
+
+func TestValidateContract_ZeroFieldsSet(t *testing.T) {
+	c := ScenarioContract{
+		Scenarios: []ScenarioAssertions{
+			{Name: "s1", Assertions: []ContractAssertion{
+				{}, // no fields set
+			}},
+		},
+	}
+	errs := ValidateContract(c)
+	if len(errs) != 1 {
+		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
+	}
+}
+
+func TestValidateContract_MultipleFieldsSet(t *testing.T) {
+	c := ScenarioContract{
+		Scenarios: []ScenarioAssertions{
+			{Name: "s1", Assertions: []ContractAssertion{
+				{
+					FileExists:    "a.go",
+					FileNotExists: "b.go",
+				},
+			}},
+		},
+	}
+	errs := ValidateContract(c)
+	if len(errs) != 1 {
+		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
+	}
+}
+
+func TestValidateContract_MixedValidAndInvalid(t *testing.T) {
+	c := ScenarioContract{
+		Scenarios: []ScenarioAssertions{
+			{
+				Name: "s1",
+				Assertions: []ContractAssertion{
+					{FileExists: "ok.go"}, // valid
+					{},                   // zero fields — invalid
+					{FileNotExists: "x"}, // valid
+				},
+			},
+			{
+				Name: "s2",
+				Assertions: []ContractAssertion{
+					{FileExists: "a.go", FileNotExists: "b.go"}, // multiple fields — invalid
+					{FileExists: "ok.go"},                       // valid
+				},
+			},
+		},
+	}
+	errs := ValidateContract(c)
+	if len(errs) != 2 {
+		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
+	}
+}
diff --git a/internal/next/runstore/events.go b/internal/next/runstore/events.go
index 09da7cbfa..1f8d30649 100644
--- a/internal/next/runstore/events.go
+++ b/internal/next/runstore/events.go
@@ -141,6 +141,21 @@ type BlockedWorktreeCleanedEvent struct {
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
+type ContractScenarioSkippedEvent struct {
+	BaseEvent
+	Reason string `json:"reason"`
+}
+
 type TerminalStateEvent struct {
 	BaseEvent
 	Status string `json:"status"`
@@ -280,6 +295,15 @@ func unmarshalEvent(data []byte) (TypedEvent, error) {
 	case "terminal_state":
 		var e TerminalStateEvent
 		ev = &e
+	case "contracts_written":
+		var e ContractsWrittenEvent
+		ev = &e
+	case "contracts_blocked":
+		var e ContractsBlockedEvent
+		ev = &e
+	case "contract_scenario_skipped":
+		var e ContractScenarioSkippedEvent
+		ev = &e
 	default:
 		return nil, fmt.Errorf("unknown event type: %s", peek.Type)
 	}
diff --git a/internal/next/runstore/events_test.go b/internal/next/runstore/events_test.go
index 7fcc35d38..9ea38f4eb 100644
--- a/internal/next/runstore/events_test.go
+++ b/internal/next/runstore/events_test.go
@@ -239,6 +239,82 @@ func TestUnmarshalEvent_BlockedWorktreeCleaned(t *testing.T) {
 	}
 }
 
+func TestContractsWrittenEvent_JSON(t *testing.T) {
+	evt := ContractsWrittenEvent{
+		BaseEvent:     BaseEvent{Type: "contracts_written", Timestamp: time.Now()},
+		ScenarioCount: 5,
+	}
+
+	data, err := json.Marshal(evt)
+	if err != nil {
+		t.Fatalf("marshal: %v", err)
+	}
+
+	var got map[string]interface{}
+	if err := json.Unmarshal(data, &got); err != nil {
+		t.Fatalf("unmarshal: %v", err)
+	}
+	if got["type"] != "contracts_written" {
+		t.Errorf("type = %v, want contracts_written", got["type"])
+	}
+	if got["scenario_count"] != float64(5) {
+		t.Errorf("scenario_count = %v, want 5", got["scenario_count"])
+	}
+}
+
+func TestContractsBlockedEvent_JSON(t *testing.T) {
+	evt := ContractsBlockedEvent{
+		BaseEvent: BaseEvent{Type: "contracts_blocked", Timestamp: time.Now()},
+		Reason:    "spec not ready",
+	}
+
+	data, err := json.Marshal(evt)
+	if err != nil {
+		t.Fatalf("marshal: %v", err)
+	}
+
+	var got map[string]interface{}
+	if err := json.Unmarshal(data, &got); err != nil {
+		t.Fatalf("unmarshal: %v", err)
+	}
+	if got["type"] != "contracts_blocked" {
+		t.Errorf("type = %v, want contracts_blocked", got["type"])
+	}
+	if got["reason"] != "spec not ready" {
+		t.Errorf("reason = %v, want 'spec not ready'", got["reason"])
+	}
+}
+
+func TestUnmarshalEvent_ContractsWritten(t *testing.T) {
+	jsonStr := `{"type":"contracts_written","timestamp":"2026-03-16T00:00:00Z","scenario_count":3}`
+	evt, err := unmarshalEvent([]byte(jsonStr))
+	if err != nil {
+		t.Fatalf("unmarshalEvent: %v", err)
+	}
+	cw, ok := evt.(*ContractsWrittenEvent)
+	if !ok {
+		t.Fatalf("expected *ContractsWrittenEvent, got %T", evt)
+	}
+	if cw.ScenarioCount != 3 {
+		t.Errorf("ScenarioCount = %d, want 3", cw.ScenarioCount)
+	}
+}
+
+func TestUnmarshalEvent_ContractsBlocked(t *testing.T) {
+	jsonStr := `{"type":"contracts_blocked","timestamp":"2026-03-16T00:00:00Z","reason":"no scenarios"}`
+	evt, err := unmarshalEvent([]byte(jsonStr))
+	if err != nil {
+		t.Fatalf("unmarshalEvent: %v", err)
+	}
+	cb, ok := evt.(*ContractsBlockedEvent)
+	if !ok {
+		t.Fatalf("expected *ContractsBlockedEvent, got %T", evt)
+	}
+	if cb.Reason != "no scenarios" {
+		t.Errorf("Reason = %q, want 'no scenarios'", cb.Reason)
+	}
+}
+
 func TestEvents_ReadAll_EmptyFile(t *testing.T) {
 	dir := t.TempDir()
 	el := NewEventLog(filepath.Join(dir, "events.jsonl"))
diff --git a/internal/next/runstore/store.go b/internal/next/runstore/store.go
index f91457ae1..075a95984 100644
--- a/internal/next/runstore/store.go
+++ b/internal/next/runstore/store.go
@@ -76,6 +76,18 @@ func (s *Store) List(projectID string) ([]*RunState, error) {
 	return result, nil
 }
 
+// ResetForNewCycle resets per-cycle gate fields on rs.
+// Fields that persist across replan cycles (e.g. ContractsWritten, ReplanContext,
+// AccumulatedCost, TotalReplans) are intentionally NOT reset here.
+func ResetForNewCycle(rs *RunState) {
+	rs.FinalValidationPassed = false
+	rs.FinalReviewPassed = false
+	rs.FinalAcceptancePassed = false
+	rs.ReviewFindings = []string{}
+	rs.AcceptanceResults = []string{}
+	// ContractsWritten is NOT reset — contracts are written once and persist across cycles.
+}
+
 // RunDir returns the directory path for a given run ID.
 func (s *Store) RunDir(runID string) string {
 	return filepath.Join(s.rootDir, "runs", runID)
diff --git a/internal/next/runstore/types.go b/internal/next/runstore/types.go
index 594c7b63a..275b45831 100644
--- a/internal/next/runstore/types.go
+++ b/internal/next/runstore/types.go
@@ -44,6 +44,7 @@ type RunState struct {
 	TotalReplans          int                    `json:"total_replans"`
 	SpecConstraints       string                 `json:"spec_constraints,omitempty"`
 	Resumed               bool                   `json:"resumed,omitempty"`
+	ContractsWritten      bool                   `json:"contracts_written"`
 }
 
 // See CLAUDE.md nil-field normalization visibility convention:
diff --git a/internal/next/specloop/contract_integration_test.go b/internal/next/specloop/contract_integration_test.go
new file mode 100644
index 000000000..37042f4a0
--- /dev/null
+++ b/internal/next/specloop/contract_integration_test.go
@@ -0,0 +1,532 @@
+package specloop_test
+
+import (
+	"context"
+	"os"
+	"path/filepath"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/next/contract"
+	"github.com/danabrams/gromit/internal/next/execpolicy"
+	"github.com/danabrams/gromit/internal/next/runstore"
+	"github.com/danabrams/gromit/internal/next/specloop"
+	"github.com/danabrams/gromit/internal/next/specloop/stages"
+	"github.com/danabrams/gromit/internal/next/validator"
+)
+
+// --- Test doubles ---
+
+type fakeContractWriter struct {
+	result *contract.ScenarioContract
+	err    error
+	calls  int
+}
+
+func (f *fakeContractWriter) WriteContracts(_ context.Context, _ []contract.SpecScenario, _ string) (*contract.ScenarioContract, error) {
+	f.calls++
+	return f.result, f.err
+}
+
+type fakeContractEvaluator struct {
+	failures []contract.ContractFailure
+}
+
+func (f *fakeContractEvaluator) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
+	return f.failures, nil
+}
+
+type callbackEvaluator struct {
+	fn func(c *contract.ScenarioContract, workDir string) []contract.ContractFailure
+}
+
+func (c *callbackEvaluator) Evaluate(_ context.Context, sc *contract.ScenarioContract, workDir string) ([]contract.ContractFailure, error) {
+	return c.fn(sc, workDir), nil
+}
+
+type fakeFinalValidator struct {
+	result validator.FinalResult
+	err    error
+}
+
+func (f *fakeFinalValidator) RunFinal(_ context.Context, _ []validator.Check, _ []validator.Check, _ string) (validator.FinalResult, error) {
+	return f.result, f.err
+}
+
+func passValidator() *fakeFinalValidator {
+	return &fakeFinalValidator{
+		result: validator.FinalResult{
+			Pass:          true,
+			AlwaysRun:     validator.CheckResults{},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+}
+
+// contractScenarioStage is a flexible mock stage for contract integration tests.
+type contractScenarioStage struct {
+	name      string
+	callCount int
+	fn        func(ctx context.Context, rs *runstore.RunState, call int) (specloop.NextAction, error)
+}
+
+func (s *contractScenarioStage) Name() string { return s.name }
+func (s *contractScenarioStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
+	s.callCount++
+	return s.fn(ctx, rs, s.callCount)
+}
+
+// --- Environment setup ---
+
+type contractTestEnv struct {
+	tmp         string
+	specPath    string
+	evidenceDir string
+	store       *runstore.Store
+	rs          *runstore.RunState
+}
+
+func setupContractEnv(t *testing.T, specContent string) *contractTestEnv {
+	t.Helper()
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := runstore.NewRunState("spec-contract", "proj-contract")
+
+	// Create run dir with spec-packet.md (required by WriteContractsStage).
+	runDir := store.RunDir(rs.RunID)
+	if err := os.MkdirAll(runDir, 0o755); err != nil {
+		t.Fatalf("create run dir: %v", err)
+	}
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("compiled spec packet"), 0o644); err != nil {
+		t.Fatalf("write spec-packet: %v", err)
+	}
+
+	// Write spec file.
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
+		t.Fatalf("write spec: %v", err)
+	}
+
+	// Create evidence dir.
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatalf("create evidence dir: %v", err)
+	}
+
+	return &contractTestEnv{
+		tmp:         tmp,
+		specPath:    specPath,
+		evidenceDir: evidenceDir,
+		store:       store,
+		rs:          rs,
+	}
+}
+
+func contractBudget(maxCycles int) *specloop.Budget {
+	return specloop.NewBudget(execpolicy.Budgets{
+		MaxSpecCycles:          maxCycles,
+		MaxRunCostUSD:          99,
+		MaxRunDurationSeconds:  3600,
+		MaxTaskDurationSeconds: 300,
+	})
+}
+
+func readyForReviewStage() *contractScenarioStage {
+	return &contractScenarioStage{
+		name: "finalize",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
+			rs.Status = runstore.StatusReadyForReview
+			return specloop.NextAction{Kind: specloop.Continue}, nil
+		},
+	}
+}
+
+const specWithContractScenarios = `# Contract Test Spec
+
+## Scenarios
+
+### Scenario: feature-works
+**When:** feature is invoked
+**Then:** output file exists
+`
+
+const specWithNoScenarios = `# Contract Test Spec
+
+## Overview
+No scenarios here.
+`
+
+// validContract is a well-formed ScenarioContract with one assertion.
+func validContract() contract.ScenarioContract {
+	return contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{
+				Name:       "feature-works",
+				Assertions: []contract.ContractAssertion{{FileExists: "output.txt"}},
+			},
+		},
+	}
+}
+
+// --- Scenario 1: Happy path — contracts pass, pipeline continues without replan ---
+
+// TestIntegration_ContractHappyPath verifies that when contracts are written and all
+// assertions pass, the pipeline continues without triggering a replan.
+func TestIntegration_ContractHappyPath(t *testing.T) {
+	env := setupContractEnv(t, specWithContractScenarios)
+
+	writer := &fakeContractWriter{result: func() *contract.ScenarioContract { c := validContract(); return &c }()}
+	evaluator := &fakeContractEvaluator{failures: nil} // all assertions pass
+
+	writeContractsStage := stages.NewWriteContractsStage(writer,
+		stages.WriteContractsStageConfig{
+			SpecPath:    env.specPath,
+			EvidenceDir: env.evidenceDir,
+			Store:       env.store,
+		}, nil, nil)
+
+	validateStage := stages.NewValidateStage(passValidator(),
+		stages.ValidateStageConfig{
+			WorkDir:     env.tmp,
+			EvidenceDir: env.evidenceDir,
+		}, nil, evaluator)
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
+		specloop.SpecLoopConfig{
+			Budget:      contractBudget(3),
+			ReplanStage: "write_contracts",
+		},
+	)
+
+	if err := loop.Run(context.Background(), env.rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if env.rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
+	}
+	if env.rs.Cycle != 1 {
+		t.Errorf("want cycle 1 (no replan), got %d", env.rs.Cycle)
+	}
+	if !env.rs.ContractsWritten {
+		t.Error("ContractsWritten should be true after write_contracts stage succeeds")
+	}
+	if writer.calls != 1 {
+		t.Errorf("expected 1 writer call, got %d", writer.calls)
+	}
+	if env.rs.TotalReplans != 0 {
+		t.Errorf("expected 0 replans on happy path, got %d", env.rs.TotalReplans)
+	}
+}
+
+// --- Scenario 2: Contract failure triggers replan, fix cycle passes ---
+
+// TestIntegration_ContractFailureTriggersReplan_ReplanStageBypassesWriteContracts verifies that a contract assertion
+// failure in ValidateStage triggers a replan, and the pipeline passes on the fix cycle.
+func TestIntegration_ContractFailureTriggersReplan_ReplanStageBypassesWriteContracts(t *testing.T) {
+	env := setupContractEnv(t, specWithContractScenarios)
+
+	writer := &fakeContractWriter{result: func() *contract.ScenarioContract { c := validContract(); return &c }()}
+
+	// First evaluation returns failures; second returns none.
+	evalCalls := 0
+	evaluator := &callbackEvaluator{
+		fn: func(_ *contract.ScenarioContract, _ string) []contract.ContractFailure {
+			evalCalls++
+			if evalCalls == 1 {
+				return []contract.ContractFailure{
+					{
+						ScenarioName:  "feature-works",
+						AssertionType: "file_exists",
+						Details:       `file "output.txt" does not exist`,
+					},
+				}
+			}
+			return nil
+		},
+	}
+
+	writeContractsStage := stages.NewWriteContractsStage(writer,
+		stages.WriteContractsStageConfig{
+			SpecPath:    env.specPath,
+			EvidenceDir: env.evidenceDir,
+			Store:       env.store,
+		}, nil, nil)
+
+	validateStage := stages.NewValidateStage(passValidator(),
+		stages.ValidateStageConfig{
+			WorkDir:     env.tmp,
+			EvidenceDir: env.evidenceDir,
+		}, nil, evaluator)
+
+	// ReplanStage: "validate" causes replan to skip write_contracts entirely because the replan
+	// stage sequence resumes at the validate stage, bypassing write_contracts. This demonstrates
+	// stage-ordering behavior, NOT the ContractsWritten idempotency guard (which is tested in
+	// TestIntegration_WriteContractsIdempotentOnReplanFromPlan with ReplanStage: "plan").
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
+		specloop.SpecLoopConfig{
+			Budget:      contractBudget(3),
+			ReplanStage: "validate",
+		},
+	)
+
+	if err := loop.Run(context.Background(), env.rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if env.rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q after replan fix, got %q", runstore.StatusReadyForReview, env.rs.Status)
+	}
+	if env.rs.Cycle != 2 {
+		t.Errorf("want cycle 2 (one replan), got %d", env.rs.Cycle)
+	}
+	if env.rs.TotalReplans != 1 {
+		t.Errorf("expected 1 replan, got %d", env.rs.TotalReplans)
+	}
+	// Contract file must have been written in cycle 1 and reused in cycle 2
+	contractPath := filepath.Join(env.evidenceDir, "scenario-contracts.yaml")
+	if _, err := os.Stat(contractPath); err != nil {
+		t.Errorf("scenario-contracts.yaml must exist after write_contracts: %v", err)
+	}
+	// Note: write_contracts is not re-run because ReplanStage=validate bypasses it in stage ordering,
+	// not because ContractsWritten=true. See TestIntegration_WriteContractsIdempotentOnReplan for AC8 coverage.
+	if writer.calls != 1 {
+		t.Errorf("expected 1 writer call (stage sequence skips write_contracts on replan), got %d", writer.calls)
+	}
+}
+
+// --- Scenario 3: WriteContracts idempotency — skipped when ContractsWritten=true ---
+
+// TestIntegration_WriteContractsIdempotentOnReplan verifies that WriteContractsStage
+// is skipped (ContractWriter not called) when ContractsWritten=true on a replan cycle.
+func TestIntegration_WriteContractsIdempotentOnReplan(t *testing.T) {
+	env := setupContractEnv(t, specWithContractScenarios)
+	env.rs.ContractsWritten = true // simulate prior cycle already wrote contracts
+
+	writer := &fakeContractWriter{} // must never be called
+
+	writeContractsStage := stages.NewWriteContractsStage(writer,
+		stages.WriteContractsStageConfig{
+			SpecPath:    env.specPath,
+			EvidenceDir: env.evidenceDir,
+			Store:       env.store,
+		}, nil, nil)
+
+	validateCallCount := 0
+	validateStage := &contractScenarioStage{
+		name: "validate",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
+			validateCallCount++
+			if validateCallCount == 1 {
+				// First call: trigger replan so write_contracts runs again on cycle 2.
+				return specloop.NextAction{
+					Kind: specloop.ReplanFrom,
+					Context: &specloop.FailureContext{
+						Failures: []string{"test failure to exercise replan"},
+						Cycle:    rs.Cycle,
+					},
+				}, nil
+			}
+			// Second call: pass.
+			rs.FinalValidationPassed = true
+			return specloop.NextAction{Kind: specloop.Continue}, nil
+		},
+	}
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
+		specloop.SpecLoopConfig{
+			Budget:      contractBudget(3),
+			ReplanStage: "write_contracts",
+		},
+	)
+
+	if err := loop.Run(context.Background(), env.rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if env.rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
+	}
+	if env.rs.Cycle != 2 {
+		t.Errorf("want cycle 2 after replan, got %d", env.rs.Cycle)
+	}
+	// Writer must never be called: ContractsWritten=true causes idempotent skip both cycles.
+	if writer.calls != 0 {
+		t.Errorf("expected 0 writer calls (idempotent when ContractsWritten=true), got %d", writer.calls)
+	}
+	if env.rs.TotalReplans != 1 {
+		t.Errorf("expected 1 replan, got %d", env.rs.TotalReplans)
+	}
+}
+
+// TestIntegration_WriteContractsIdempotentOnReplanFromPlan verifies that when ContractsWritten=true
+// and replan starts from "plan" (reaching write_contracts stage), the stage is skipped entirely
+// due to the ContractsWritten guard, not due to bypassing the stage. This exercises AC8: the
+// idempotency guard that prevents re-writing contracts when they already exist.
+func TestIntegration_WriteContractsIdempotentOnReplanFromPlan(t *testing.T) {
+	env := setupContractEnv(t, specWithContractScenarios)
+	env.rs.ContractsWritten = true // simulate prior cycle already wrote contracts
+
+	writer := &fakeContractWriter{} // must never be called
+
+	writeContractsStage := stages.NewWriteContractsStage(writer,
+		stages.WriteContractsStageConfig{
+			SpecPath:    env.specPath,
+			EvidenceDir: env.evidenceDir,
+			Store:       env.store,
+		}, nil, nil)
+
+	validateCallCount := 0
+	validateStage := &contractScenarioStage{
+		name: "validate",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
+			validateCallCount++
+			if validateCallCount == 1 {
+				// First call: trigger replan from plan stage.
+				return specloop.NextAction{
+					Kind: specloop.ReplanFrom,
+					Context: &specloop.FailureContext{
+						Failures: []string{"test failure to exercise replan from plan"},
+						Cycle:    rs.Cycle,
+					},
+				}, nil
+			}
+			// Second call: pass.
+			rs.FinalValidationPassed = true
+			return specloop.NextAction{Kind: specloop.Continue}, nil
+		},
+	}
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
+		specloop.SpecLoopConfig{
+			Budget:      contractBudget(3),
+			ReplanStage: "plan",
+		},
+	)
+
+	if err := loop.Run(context.Background(), env.rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if env.rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
+	}
+	if env.rs.Cycle != 2 {
+		t.Errorf("want cycle 2 after replan, got %d", env.rs.Cycle)
+	}
+	// Writer must never be called: ContractsWritten=true causes idempotent skip on replan.
+	// This is AC8 behavior — write_contracts is guarded by ContractsWritten, not bypassed by ReplanStage.
+	if writer.calls != 0 {
+		t.Errorf("expected 0 writer calls (idempotent when ContractsWritten=true), got %d", writer.calls)
+	}
+	if env.rs.TotalReplans != 1 {
+		t.Errorf("expected 1 replan, got %d", env.rs.TotalReplans)
+	}
+}
+
+// --- Scenario 4: No scenarios — WriteContracts is no-op, Validate skips contracts ---
+
+// TestIntegration_NoScenariosWriteContractsNoOp verifies that when the spec has no
+// scenarios, WriteContractsStage is a no-op (writer not called, no contract file created)
+// and ValidateStage proceeds without contract evaluation.
+func TestIntegration_NoScenariosWriteContractsNoOp(t *testing.T) {
+	env := setupContractEnv(t, specWithNoScenarios)
+
+	writer := &fakeContractWriter{} // must never be called
+	evaluator := &fakeContractEvaluator{failures: nil}
+
+	writeContractsStage := stages.NewWriteContractsStage(writer,
+		stages.WriteContractsStageConfig{
+			SpecPath:    env.specPath,
+			EvidenceDir: env.evidenceDir,
+			Store:       env.store,
+		}, nil, nil)
+
+	validateStage := stages.NewValidateStage(passValidator(),
+		stages.ValidateStageConfig{
+			WorkDir:     env.tmp,
+			EvidenceDir: env.evidenceDir,
+		}, nil, evaluator)
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, validateStage, readyForReviewStage()},
+		specloop.SpecLoopConfig{
+			Budget:      contractBudget(3),
+			ReplanStage: "write_contracts",
+		},
+	)
+
+	if err := loop.Run(context.Background(), env.rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if env.rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, env.rs.Status)
+	}
+	if env.rs.Cycle != 1 {
+		t.Errorf("want cycle 1 (no replan), got %d", env.rs.Cycle)
+	}
+	if writer.calls != 0 {
+		t.Errorf("expected 0 writer calls (no scenarios), got %d", writer.calls)
+	}
+	// No contract file should be written when spec has no scenarios.
+	contractPath := filepath.Join(env.evidenceDir, "scenario-contracts.yaml")
+	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
+		t.Error("expected no scenario-contracts.yaml for no-scenarios spec")
+	}
+	// ContractsWritten remains false when there are no scenarios to write.
+	if env.rs.ContractsWritten {
+		t.Error("ContractsWritten should be false when spec has no scenarios")
+	}
+}
+
+// --- Scenario 5: Missing contract file at Validate time — skipped silently ---
+
+// TestIntegration_MissingContractFileSkippedSilently verifies that when EvidenceDir is
+// configured but scenario-contracts.yaml does not exist at validate time, the stage
+// proceeds silently without error and without triggering a replan.
+func TestIntegration_MissingContractFileSkippedSilently(t *testing.T) {
+	env := setupContractEnv(t, specWithContractScenarios)
+
+	// Evaluator is configured with failures but must never be called — the missing
+	// file causes the evaluation to be skipped entirely.
+	evaluator := &fakeContractEvaluator{
+		failures: []contract.ContractFailure{
+			{ScenarioName: "feature-works", AssertionType: "file_exists", Details: "should not appear"},
+		},
+	}
+
+	// ValidateStage with EvidenceDir set, but no scenario-contracts.yaml written.
+	validateStage := stages.NewValidateStage(passValidator(),
+		stages.ValidateStageConfig{
+			WorkDir:     env.tmp,
+			EvidenceDir: env.evidenceDir, // dir exists but contains no contract file
+		}, nil, evaluator)
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{validateStage, readyForReviewStage()},
+		specloop.SpecLoopConfig{
+			Budget:      contractBudget(3),
+			ReplanStage: "validate",
+		},
+	)
+
+	if err := loop.Run(context.Background(), env.rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	// Missing contract file must not trigger a replan — silently skipped.
+	if env.rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q (missing contract file silently skipped), got %q",
+			runstore.StatusReadyForReview, env.rs.Status)
+	}
+	if env.rs.Cycle != 1 {
+		t.Errorf("want cycle 1 (no replan when contract file missing), got %d", env.rs.Cycle)
+	}
+	if env.rs.TotalReplans != 0 {
+		t.Errorf("expected 0 replans when contract file absent, got %d", env.rs.TotalReplans)
+	}
+}
diff --git a/internal/next/specloop/specloop.go b/internal/next/specloop/specloop.go
index 346a02e67..efd64ace0 100644
--- a/internal/next/specloop/specloop.go
+++ b/internal/next/specloop/specloop.go
@@ -39,15 +39,9 @@ func (sl *SpecLoop) Run(ctx context.Context, rs *runstore.RunState) error {
 	for cycle := 0; cycle < maxCycles; cycle++ {
 		rs.Cycle = cycle + 1
 
-		// Reset gate booleans and review/acceptance fields at cycle start.
-		// NOTE: ReplanContext is NOT reset here — it is set at the end of the
-		// previous cycle (after replan is triggered) and consumed by PlanStage
-		// at the start of this cycle to determine isFixCycle.
-		rs.FinalValidationPassed = false
-		rs.FinalReviewPassed = false
-		rs.FinalAcceptancePassed = false
-		rs.ReviewFindings = []string{}
-		rs.AcceptanceResults = []string{}
+		// Reset per-cycle gate fields. ReplanContext and ContractsWritten are
+		// intentionally preserved — see runstore.ResetForNewCycle for the full list.
+		runstore.ResetForNewCycle(rs)
 
 		startIdx := 0
 		if cycle > 0 && sl.config.ReplanStage != "" {
diff --git a/internal/next/specloop/specloop_test.go b/internal/next/specloop/specloop_test.go
index f7e34bfce..f2b2dd85e 100644
--- a/internal/next/specloop/specloop_test.go
+++ b/internal/next/specloop/specloop_test.go
@@ -678,6 +678,33 @@ func TestSpecLoop_ReviewReplan_RunsAcceptOnExhaustion(t *testing.T) {
 	}
 }
 
+func TestContractsWritten_PersistedAcrossReplanCycles(t *testing.T) {
+	// ContractsWritten must NOT be reset in the per-cycle reset block (specloop.go:46).
+	// WriteContracts sets it on cycle 1; on replan cycles it must still be true
+	// so that WriteContracts can short-circuit as a no-op.
+	rs := runstore.NewRunState("test-spec", "test-project")
+	rs.ContractsWritten = true
+
+	var snapContractsWritten bool
+
+	captureStage := &mockStage{
+		name: "plan",
+		runFn: func(_ context.Context, rs *runstore.RunState) (NextAction, error) {
+			snapContractsWritten = rs.ContractsWritten
+			return NextAction{Kind: Continue}, nil
+		},
+	}
+
+	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxTaskDurationSeconds: 300, MaxRunDurationSeconds: 3600, MaxRunCostUSD: 50.0})
+	loop := NewSpecLoop([]Stage{captureStage}, SpecLoopConfig{Budget: budget})
+
+	loop.Run(context.Background(), rs)
+
+	if !snapContractsWritten {
+		t.Error("ContractsWritten should NOT be reset at cycle start — it must persist across replan cycles")
+	}
+}
+
 func TestSpecLoop_CycleExhaustion_SetsBlockerSummaryFromReplanContext(t *testing.T) {
 	budget := NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99})
 
diff --git a/internal/next/specloop/stages/plan.go b/internal/next/specloop/stages/plan.go
index 226b2f593..f75d733fb 100644
--- a/internal/next/specloop/stages/plan.go
+++ b/internal/next/specloop/stages/plan.go
@@ -73,10 +73,9 @@ func (s *PlanStage) Name() string { return "plan" }
 // Run executes the plan stage.
 func (s *PlanStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
 	isFixCycle := rs.Cycle > 1 && len(rs.ReplanContext) > 0
-
-	// Skip planning when this is a resumed run with existing tasks and no
-	// replan context. The Resumed flag is set by exec.go when --resume is used.
 	if rs.Resumed && len(rs.Tasks) > 0 && !isFixCycle {
+		// Resumed run with existing tasks: skip planning on the first cycle so
+		// execution continues from where it left off without overwriting tasks.
 		return specloop.NextAction{Kind: specloop.Continue}, nil
 	}
 
diff --git a/internal/next/specloop/stages/validate.go b/internal/next/specloop/stages/validate.go
index 59f8915f6..2a85b6cb8 100644
--- a/internal/next/specloop/stages/validate.go
+++ b/internal/next/specloop/stages/validate.go
@@ -2,9 +2,13 @@ package stages
 
 import (
 	"context"
+	"errors"
 	"fmt"
+	"os"
+	"path/filepath"
 	"time"
 
+	"github.com/danabrams/gromit/internal/next/contract"
 	"github.com/danabrams/gromit/internal/next/runstore"
 	"github.com/danabrams/gromit/internal/next/specloop"
 	"github.com/danabrams/gromit/internal/next/validator"
@@ -20,18 +24,21 @@ type ValidateStageConfig struct {
 	AlwaysRun     []validator.Check
 	ProjectChecks []validator.Check
 	WorkDir       string
+	EvidenceDir   string
 }
 
 // ValidateStage runs final validation checks.
 type ValidateStage struct {
-	validator FinalValidator
-	cfg       ValidateStageConfig
-	eventLog  *runstore.EventLog
+	validator         FinalValidator
+	contractEvaluator contract.ContractEvaluator
+	cfg               ValidateStageConfig
+	eventLog          *runstore.EventLog
 }
 
-// NewValidateStage creates a new ValidateStage.
-func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runstore.EventLog) *ValidateStage {
-	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog}
+// NewValidateStage creates a new ValidateStage. An optional ContractEvaluator may be
+// provided; if nil, contract checking is skipped.
+func NewValidateStage(v FinalValidator, cfg ValidateStageConfig, eventLog *runstore.EventLog, evaluator contract.ContractEvaluator) *ValidateStage {
+	return &ValidateStage{validator: v, cfg: cfg, eventLog: eventLog, contractEvaluator: evaluator}
 }
 
 // Name returns the stage name.
@@ -43,37 +50,67 @@ func (s *ValidateStage) Run(ctx context.Context, rs *runstore.RunState) (specloo
 	if rs.WorktreePath != "" {
 		workDir = rs.WorktreePath
 	}
+
+	// Collect contract failures first (if EvidenceDir is configured and file exists).
+	var failures []string
+	if s.cfg.EvidenceDir != "" && s.contractEvaluator != nil {
+		contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")
+		data, err := os.ReadFile(contractPath)
+		if err != nil && !errors.Is(err, os.ErrNotExist) {
+			return specloop.NextAction{}, fmt.Errorf("read scenario-contracts.yaml: %w", err)
+		}
+		if err == nil {
+			sc, err := contract.ParseContractYAML(string(data))
+			if err != nil {
+				return specloop.NextAction{}, fmt.Errorf("parse scenario-contracts.yaml: %w", err)
+			}
+			contractFailures, err := s.contractEvaluator.Evaluate(ctx, &sc, workDir)
+			if err != nil {
+				return specloop.NextAction{}, fmt.Errorf("evaluate contracts: %w", err)
+			}
+			for _, f := range contractFailures {
+				failures = append(failures, fmt.Sprintf("contract:%s — %s failed: %s", f.ScenarioName, f.AssertionType, f.Details))
+			}
+		}
+	}
+
+	// Run shell checks regardless of contract results.
 	result, err := s.validator.RunFinal(ctx, s.cfg.AlwaysRun, s.cfg.ProjectChecks, workDir)
 	if err != nil {
 		return specloop.NextAction{}, fmt.Errorf("final validation: %w", err)
 	}
 
 	// Store validation result summary in RunState for EvidenceStage (L3)
-	validationSummary := fmt.Sprintf("pass=%v", result.Pass)
-	rs.LastValidationResult = &validationSummary
 	rs.LastFinalValidation = &result
 
-	// Emit final_validation_result event
+	// Collect shell check failures.
+	for _, cr := range result.AlwaysRun.FailedChecks() {
+		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, cr.Output))
+	}
+	for _, cr := range result.ProjectChecks.FailedChecks() {
+		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, cr.Output))
+	}
+
+	// Determine final validation status after collecting ALL failures (contract + shell).
+	finalPassed := len(failures) == 0 && result.Pass
+
+	// Store validation result summary reflecting actual final status.
+	validationSummary := fmt.Sprintf("pass=%v", finalPassed)
+	rs.LastValidationResult = &validationSummary
+
+	// Emit final_validation_result event after all failures are collected.
 	if s.eventLog != nil {
 		s.eventLog.Append(runstore.FinalValidationResultEvent{
 			BaseEvent: runstore.BaseEvent{Type: "final_validation_result", Timestamp: time.Now()},
-			Passed:    result.Pass,
+			Passed:    finalPassed,
 		})
 	}
 
-	if result.Pass {
+	if finalPassed {
 		rs.FinalValidationPassed = true
 		return specloop.NextAction{Kind: specloop.Continue}, nil
 	}
 
-	// Collect failure details
-	var failures []string
-	for _, cr := range result.AlwaysRun.FailedChecks() {
-		failures = append(failures, fmt.Sprintf("always-run check %q failed: %s", cr.Name, cr.Output))
-	}
-	for _, cr := range result.ProjectChecks.FailedChecks() {
-		failures = append(failures, fmt.Sprintf("project check %q failed: %s", cr.Name, cr.Output))
-	}
 	if len(failures) == 0 {
 		failures = []string{"validation failed"}
 	}
diff --git a/internal/next/specloop/stages/validate_test.go b/internal/next/specloop/stages/validate_test.go
index c2bf6dc73..4e8636c63 100644
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
@@ -18,6 +22,14 @@ func (f *fakeValidator) RunFinal(ctx context.Context, alwaysRun []validator.Chec
 	return f.result, f.err
 }
 
+type fakeContractEvaluator struct {
+	failures []contract.ContractFailure
+}
+
+func (f *fakeContractEvaluator) Evaluate(_ context.Context, _ *contract.ScenarioContract, _ string) ([]contract.ContractFailure, error) {
+	return f.failures, nil
+}
+
 // Verify ValidateStage satisfies the Stage interface.
 var _ specloop.Stage = (*ValidateStage)(nil)
 
@@ -34,7 +46,7 @@ func TestValidateStage_AllPass_Continue(t *testing.T) {
 		},
 	}
 
-	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil)
+	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil, nil)
 
 	if stage.Name() != "validate" {
 		t.Fatalf("expected name 'validate', got %q", stage.Name())
@@ -66,7 +78,7 @@ func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
 		},
 	}
 
-	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil)
+	stage := NewValidateStage(v, ValidateStageConfig{WorkDir: "/tmp/work"}, nil, nil)
 
 	rs := runstore.NewRunState("spec-001", "proj-001")
 	rs.Cycle = 1
@@ -87,3 +99,136 @@ func TestValidateStage_Failure_ReplanFrom(t *testing.T) {
 		t.Fatal("expected failures to be non-empty")
 	}
 }
+
+// TestValidateStage_MissingContractFile verifies that when EvidenceDir is set but
+// scenario-contracts.yaml does not exist, the stage proceeds silently without error.
+func TestValidateStage_MissingContractFile(t *testing.T) {
+	dir := t.TempDir()
+
+	v := &fakeValidator{
+		result: validator.FinalResult{
+			Pass:          true,
+			AlwaysRun:     validator.CheckResults{},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+	evaluator := &fakeContractEvaluator{}
+
+	stage := NewValidateStage(v, ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir, // dir exists but contains no scenario-contracts.yaml
+	}, nil, evaluator)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error when contract file missing: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue when contract file missing, got %v", action.Kind)
+	}
+}
+
+// TestValidateStage_ContractFailures verifies that contract assertion failures are
+// collected and reported with the format "contract:<scenario-name> — <assertion-type> failed: <details>".
+func TestValidateStage_ContractFailures(t *testing.T) {
+	dir := t.TempDir()
+
+	// Write a minimal scenario-contracts.yaml to the evidence dir.
+	yaml := `scenarios:
+  - name: subtract-works
+    assertions:
+      - file_exists: result.txt
+`
+	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(yaml), 0o644); err != nil {
+		t.Fatalf("write contract file: %v", err)
+	}
+
+	v := &fakeValidator{
+		result: validator.FinalResult{
+			Pass:          true,
+			AlwaysRun:     validator.CheckResults{},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+	evaluator := &fakeContractEvaluator{
+		failures: []contract.ContractFailure{
+			{ScenarioName: "subtract-works", AssertionType: "file_exists", Details: `file "result.txt" does not exist`},
+		},
+	}
+
+	stage := NewValidateStage(v, ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+	}, nil, evaluator)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.ReplanFrom {
+		t.Fatalf("expected ReplanFrom due to contract failure, got %v", action.Kind)
+	}
+	if action.Context == nil {
+		t.Fatal("expected FailureContext to be non-nil")
+	}
+	if len(action.Context.Failures) == 0 {
+		t.Fatal("expected failures to be non-empty")
+	}
+	want := `contract:subtract-works — file_exists failed: file "result.txt" does not exist`
+	if action.Context.Failures[0] != want {
+		t.Fatalf("expected failure %q, got %q", want, action.Context.Failures[0])
+	}
+}
+
+// TestValidateStage_ContractAndShellFailures verifies that contract failures are collected
+// first and then shell check failures are appended, both ending up in ReplanFrom failures.
+func TestValidateStage_ContractAndShellFailures(t *testing.T) {
+	dir := t.TempDir()
+
+	yaml := `scenarios:
+  - name: add-works
+    assertions:
+      - file_exists: out.txt
+`
+	if err := os.WriteFile(filepath.Join(dir, "scenario-contracts.yaml"), []byte(yaml), 0o644); err != nil {
+		t.Fatalf("write contract file: %v", err)
+	}
+
+	v := &fakeValidator{
+		result: validator.FinalResult{
+			Pass: false,
+			AlwaysRun: validator.CheckResults{
+				Results: []validator.CheckResult{{Name: "test", Pass: false, Output: "FAIL"}},
+			},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+	evaluator := &fakeContractEvaluator{
+		failures: []contract.ContractFailure{
+			{ScenarioName: "add-works", AssertionType: "file_exists", Details: `file "out.txt" does not exist`},
+		},
+	}
+
+	stage := NewValidateStage(v, ValidateStageConfig{
+		WorkDir:     "/tmp/work",
+		EvidenceDir: dir,
+	}, nil, evaluator)
+
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.ReplanFrom {
+		t.Fatalf("expected ReplanFrom, got %v", action.Kind)
+	}
+	// contract failure should be first
+	if len(action.Context.Failures) < 2 {
+		t.Fatalf("expected at least 2 failures (contract + shell), got %d", len(action.Context.Failures))
+	}
+	if !strings.HasPrefix(action.Context.Failures[0], "contract:") {
+		t.Fatalf("expected first failure to be contract failure, got %q", action.Context.Failures[0])
+	}
+}
diff --git a/internal/next/specloop/stages/write_contracts.go b/internal/next/specloop/stages/write_contracts.go
new file mode 100644
index 000000000..55327fa81
--- /dev/null
+++ b/internal/next/specloop/stages/write_contracts.go
@@ -0,0 +1,186 @@
+package stages
+
+import (
+	"context"
+	"fmt"
+	"os"
+	"path/filepath"
+	"strings"
+	"time"
+
+	"github.com/danabrams/gromit/internal/next/contract"
+	"github.com/danabrams/gromit/internal/next/runstore"
+	"github.com/danabrams/gromit/internal/next/specloop"
+	"gopkg.in/yaml.v3"
+)
+
+// WriteContractsStageConfig configures the WriteContractsStage.
+type WriteContractsStageConfig struct {
+	// SpecPath is the path to the raw spec markdown file.
+	SpecPath string
+	// EvidenceDir is the directory where scenario-contracts.yaml will be written.
+	EvidenceDir string
+	// Store provides access to run storage operations.
+	Store *runstore.Store
+}
+
+// WriteContractsStage translates spec scenarios into declarative contract assertions
+// before implementation begins. It is a no-op (idempotent) if ContractsWritten is
+// already true on the RunState. Uses Sonnet (P1) model tier.
+type WriteContractsStage struct {
+	writer   contract.ContractWriter
+	cfg      WriteContractsStageConfig
+	budget   *specloop.Budget
+	eventLog *runstore.EventLog
+}
+
+// NewWriteContractsStage creates a new WriteContractsStage.
+func NewWriteContractsStage(writer contract.ContractWriter, cfg WriteContractsStageConfig, budget *specloop.Budget, eventLog *runstore.EventLog) *WriteContractsStage {
+	return &WriteContractsStage{
+		writer:   writer,
+		cfg:      cfg,
+		budget:   budget,
+		eventLog: eventLog,
+	}
+}
+
+// Name returns the stage name.
+func (s *WriteContractsStage) Name() string { return "write_contracts" }
+
+// Run executes the write-contracts stage.
+func (s *WriteContractsStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
+	// Idempotency: if contracts are already written (e.g. on replan cycle), skip.
+	if rs.ContractsWritten {
+		return specloop.NextAction{Kind: specloop.Continue}, nil
+	}
+
+	// Read raw spec markdown to parse scenarios.
+	specBytes, err := os.ReadFile(s.cfg.SpecPath)
+	if err != nil {
+		return specloop.NextAction{}, fmt.Errorf("read spec file: %w", err)
+	}
+
+	scenarios, skipped, err := contract.ParseScenarios(string(specBytes))
+	if err != nil {
+		return specloop.NextAction{}, fmt.Errorf("parse scenarios: %w", err)
+	}
+	for _, reason := range skipped {
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.ContractScenarioSkippedEvent{
+				BaseEvent: runstore.BaseEvent{Type: "contract_scenario_skipped", Timestamp: time.Now()},
+				Reason:    reason,
+			})
+		}
+	}
+
+	// No scenarios — no-op.
+	if len(scenarios) == 0 {
+		return specloop.NextAction{Kind: specloop.Continue}, nil
+	}
+
+	// Read compiled spec packet for additional context.
+	runDir := s.cfg.Store.RunDir(rs.RunID)
+	specPacketBytes, err := os.ReadFile(filepath.Join(runDir, "spec-packet.md"))
+	if err != nil {
+		return specloop.NextAction{}, fmt.Errorf("read spec packet: %w", err)
+	}
+
+	// Budget check before LLM invocation.
+	if s.budget != nil && s.budget.Exceeded() {
+		reason := "budget exhausted: " + s.budget.Reason()
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.ContractsBlockedEvent{
+				BaseEvent: runstore.BaseEvent{Type: "contracts_blocked", Timestamp: time.Now()},
+				Reason:    reason,
+			})
+		}
+		return specloop.NextAction{
+			Kind: specloop.Blocked,
+			Context: &specloop.FailureContext{
+				Failures: []string{reason},
+				Cycle:    rs.Cycle,
+			},
+		}, nil
+	}
+
+	specPacket := string(specPacketBytes)
+
+	// Retry loop: up to 3 total attempts (1 initial + 2 retries).
+	const maxAttempts = 3
+	var (
+		result           *contract.ScenarioContract
+		validationErrors []string
+		lastErr          error
+	)
+	for attempt := 0; attempt < maxAttempts; attempt++ {
+		validationErrors = nil
+		result, lastErr = s.writer.WriteContracts(ctx, scenarios, specPacket)
+		if lastErr != nil {
+			// Infrastructure/LLM error; prepend error context to specPacket for next attempt.
+			specPacket = "# Prior LLM Error\n" + lastErr.Error() + "\n\n" + string(specPacketBytes)
+			continue
+		}
+		if result == nil {
+			lastErr = fmt.Errorf("writer returned nil contract")
+			specPacket = "# Prior LLM Error\n" + lastErr.Error() + "\n\n" + string(specPacketBytes)
+			continue
+		}
+
+		validationErrors = contract.ValidateContract(*result)
+		if len(validationErrors) == 0 {
+			// Valid output — break out of retry loop.
+			lastErr = nil
+			break
+		}
+
+		// Prepend validation errors and valid assertion keys to specPacket for next attempt.
+		const validKeys = "Valid assertion keys: file_exists, file_contains, file_not_modified, file_not_exists, file_not_contains"
+		specPacket = "# Prior Validation Errors\n" + strings.Join(validationErrors, "\n") + "\n\n# Valid Assertion Keys\n" + validKeys + "\n\n" + string(specPacketBytes)
+	}
+
+	// Determine terminal failure.
+	if lastErr != nil || len(validationErrors) > 0 {
+		reason := "contract generation failed after retries"
+		if lastErr != nil {
+			reason = fmt.Sprintf("contract writer error: %v", lastErr)
+		} else if len(validationErrors) > 0 {
+			reason = fmt.Sprintf("contract validation failed: %s", strings.Join(validationErrors, "; "))
+		}
+
+		if s.eventLog != nil {
+			s.eventLog.Append(runstore.ContractsBlockedEvent{
+				BaseEvent: runstore.BaseEvent{Type: "contracts_blocked", Timestamp: time.Now()},
+				Reason:    reason,
+			})
+		}
+		return specloop.NextAction{
+			Kind: specloop.Blocked,
+			Context: &specloop.FailureContext{
+				Failures: []string{reason},
+				Cycle:    rs.Cycle,
+			},
+		}, nil
+	}
+
+	// Write scenario-contracts.yaml to EvidenceDir.
+	contractBytes, err := yaml.Marshal(*result)
+	if err != nil {
+		return specloop.NextAction{}, fmt.Errorf("marshal contracts: %w", err)
+	}
+	contractPath := filepath.Join(s.cfg.EvidenceDir, "scenario-contracts.yaml")
+	if err := os.WriteFile(contractPath, contractBytes, 0o644); err != nil {
+		return specloop.NextAction{}, fmt.Errorf("write contracts file: %w", err)
+	}
+
+	// Set flag and emit success event.
+	rs.ContractsWritten = true
+
+	if s.eventLog != nil {
+		s.eventLog.Append(runstore.ContractsWrittenEvent{
+			BaseEvent:     runstore.BaseEvent{Type: "contracts_written", Timestamp: time.Now()},
+			ScenarioCount: len(result.Scenarios),
+		})
+	}
+
+	return specloop.NextAction{Kind: specloop.Continue}, nil
+}
diff --git a/internal/next/specloop/stages/write_contracts_integration_test.go b/internal/next/specloop/stages/write_contracts_integration_test.go
new file mode 100644
index 000000000..66b3a59c7
--- /dev/null
+++ b/internal/next/specloop/stages/write_contracts_integration_test.go
@@ -0,0 +1,283 @@
+package stages
+
+import (
+	"context"
+	"os"
+	"path/filepath"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/next/contract"
+	"github.com/danabrams/gromit/internal/next/execpolicy"
+	"github.com/danabrams/gromit/internal/next/runstore"
+	"github.com/danabrams/gromit/internal/next/specloop"
+	"github.com/danabrams/gromit/internal/next/validator"
+)
+
+// callbackStage is a flexible mock stage for write_contracts integration tests.
+type callbackStage struct {
+	name      string
+	callCount int
+	fn        func(ctx context.Context, rs *runstore.RunState, call int) (specloop.NextAction, error)
+}
+
+func (s *callbackStage) Name() string { return s.name }
+func (s *callbackStage) Run(ctx context.Context, rs *runstore.RunState) (specloop.NextAction, error) {
+	s.callCount++
+	return s.fn(ctx, rs, s.callCount)
+}
+
+func integrationBudget(maxCycles int) *specloop.Budget {
+	return specloop.NewBudget(execpolicy.Budgets{
+		MaxSpecCycles:          maxCycles,
+		MaxRunCostUSD:          99,
+		MaxRunDurationSeconds:  3600,
+		MaxTaskDurationSeconds: 300,
+	})
+}
+
+func passingValidator() *fakeValidator {
+	return &fakeValidator{
+		result: validator.FinalResult{
+			Pass:          true,
+			AlwaysRun:     validator.CheckResults{},
+			ProjectChecks: validator.CheckResults{},
+		},
+	}
+}
+
+func finalizeStage() *callbackStage {
+	return &callbackStage{
+		name: "finalize",
+		fn: func(_ context.Context, rs *runstore.RunState, _ int) (specloop.NextAction, error) {
+			rs.Status = runstore.StatusReadyForReview
+			return specloop.NextAction{Kind: specloop.Continue}, nil
+		},
+	}
+}
+
+// setupIntegrationEnv creates a temp dir with run dir, spec file, and evidence dir.
+func setupIntegrationEnv(t *testing.T, specContent string) (tmp, specPath, evidenceDir string, store *runstore.Store, rs *runstore.RunState) {
+	t.Helper()
+	tmp = t.TempDir()
+	store = runstore.NewStore(tmp)
+	rs = runstore.NewRunState("spec-wc-integ", "proj-wc-integ")
+
+	runDir := store.RunDir(rs.RunID)
+	if err := os.MkdirAll(runDir, 0o755); err != nil {
+		t.Fatalf("create run dir: %v", err)
+	}
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("compiled spec packet"), 0o644); err != nil {
+		t.Fatalf("write spec-packet: %v", err)
+	}
+
+	specPath = filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
+		t.Fatalf("write spec: %v", err)
+	}
+
+	evidenceDir = filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatalf("create evidence dir: %v", err)
+	}
+	return
+}
+
+// fileContainsContract returns a ScenarioContract asserting "output.txt" contains "expected content".
+func fileContainsContract() contract.ScenarioContract {
+	return contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{
+				Name: "add-works",
+				Assertions: []contract.ContractAssertion{
+					{
+						FileContains: &contract.FileContainsAssertion{
+							Path:    "output.txt",
+							Pattern: "expected content",
+						},
+					},
+				},
+			},
+		},
+	}
+}
+
+// TestIntegration_WriteContracts_FullPipelineWithReplan exercises the full flow:
+// (1) WriteContracts produces a file_contains contract,
+// (2) Execute creates the file WITHOUT the required content,
+// (3) Validate detects failure and triggers ReplanFrom,
+// (4) On replan cycle, WriteContracts is a no-op (ContractsWritten=true),
+// (5) Execute fix cycle writes correct content and Validate passes.
+func TestIntegration_WriteContracts_FullPipelineWithReplan(t *testing.T) {
+	tmp, specPath, evidenceDir, store, rs := setupIntegrationEnv(t, specWithScenarios)
+
+	c := fileContainsContract()
+	writer := &fakeContractWriter{result: &c}
+
+	// Execute stage: cycle 1 writes wrong content; cycle 2 writes correct content.
+	outputPath := filepath.Join(tmp, "output.txt")
+	executeStage := &callbackStage{
+		name: "execute",
+		fn: func(_ context.Context, _ *runstore.RunState, call int) (specloop.NextAction, error) {
+			content := "wrong content"
+			if call > 1 {
+				content = "expected content"
+			}
+			if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
+				return specloop.NextAction{}, err
+			}
+			return specloop.NextAction{Kind: specloop.Continue}, nil
+		},
+	}
+
+	// Use the real evaluator so filesystem state drives the contract result.
+	evaluator := &contract.DefaultContractEvaluator{}
+
+	writeContractsStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}, nil, nil)
+
+	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
+		WorkDir:     tmp,
+		EvidenceDir: evidenceDir,
+	}, nil, evaluator)
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, executeStage, validateStage, finalizeStage()},
+		specloop.SpecLoopConfig{
+			Budget:      integrationBudget(3),
+			ReplanStage: "write_contracts",
+		},
+	)
+
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
+	}
+	if rs.Cycle != 2 {
+		t.Errorf("want cycle 2 (one replan), got %d", rs.Cycle)
+	}
+	if rs.TotalReplans != 1 {
+		t.Errorf("expected 1 replan, got %d", rs.TotalReplans)
+	}
+	// Writer called exactly once — ContractsWritten=true causes idempotent skip on replan cycle.
+	if writer.calls != 1 {
+		t.Errorf("expected 1 writer call (idempotent on replan), got %d", writer.calls)
+	}
+	if !rs.ContractsWritten {
+		t.Error("ContractsWritten should be true after write_contracts stage")
+	}
+	// Execute was called twice: once broken, once fixed.
+	if executeStage.callCount != 2 {
+		t.Errorf("expected execute called 2 times, got %d", executeStage.callCount)
+	}
+	// Contract file must persist in the evidence dir.
+	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
+	if _, err := os.Stat(contractPath); err != nil {
+		t.Errorf("scenario-contracts.yaml must exist after write_contracts: %v", err)
+	}
+}
+
+// TestIntegration_WriteContracts_NoScenariosNoContractFile verifies that a spec
+// with no scenarios causes WriteContractsStage to be a no-op: the ContractWriter
+// is never called and no contract file is created. ValidateStage proceeds without
+// contract evaluation and the pipeline completes without a replan.
+func TestIntegration_WriteContracts_NoScenariosNoContractFile(t *testing.T) {
+	tmp, specPath, evidenceDir, store, rs := setupIntegrationEnv(t, specWithoutScenarios)
+
+	writer := &fakeContractWriter{} // must never be called
+	evaluator := &fakeContractEvaluator{failures: nil}
+
+	writeContractsStage := NewWriteContractsStage(writer, WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}, nil, nil)
+
+	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
+		WorkDir:     tmp,
+		EvidenceDir: evidenceDir,
+	}, nil, evaluator)
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{writeContractsStage, validateStage, finalizeStage()},
+		specloop.SpecLoopConfig{
+			Budget:      integrationBudget(3),
+			ReplanStage: "write_contracts",
+		},
+	)
+
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q, got %q", runstore.StatusReadyForReview, rs.Status)
+	}
+	if rs.Cycle != 1 {
+		t.Errorf("want cycle 1 (no replan), got %d", rs.Cycle)
+	}
+	if writer.calls != 0 {
+		t.Errorf("expected 0 writer calls for no-scenarios spec, got %d", writer.calls)
+	}
+	if rs.ContractsWritten {
+		t.Error("ContractsWritten should be false when spec has no scenarios")
+	}
+	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
+	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
+		t.Error("expected no scenario-contracts.yaml when spec has no scenarios")
+	}
+}
+
+// TestIntegration_WriteContracts_MissingContractFileGraceful verifies that when
+// EvidenceDir is configured but scenario-contracts.yaml is absent at validate time,
+// ValidateStage proceeds silently without error and without triggering a replan.
+func TestIntegration_WriteContracts_MissingContractFileGraceful(t *testing.T) {
+	tmp := t.TempDir()
+	rs := runstore.NewRunState("spec-missing-contract", "proj-missing")
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatalf("create evidence dir: %v", err)
+	}
+
+	// Evaluator is configured with failures but must never be called — the missing
+	// contract file causes evaluation to be skipped entirely.
+	evaluator := &fakeContractEvaluator{
+		failures: []contract.ContractFailure{
+			{ScenarioName: "any", AssertionType: "file_exists", Details: "should not appear"},
+		},
+	}
+
+	validateStage := NewValidateStage(passingValidator(), ValidateStageConfig{
+		WorkDir:     tmp,
+		EvidenceDir: evidenceDir, // dir exists but scenario-contracts.yaml is absent
+	}, nil, evaluator)
+
+	loop := specloop.NewSpecLoop(
+		[]specloop.Stage{validateStage, finalizeStage()},
+		specloop.SpecLoopConfig{
+			Budget:      integrationBudget(3),
+			ReplanStage: "validate",
+		},
+	)
+
+	if err := loop.Run(context.Background(), rs); err != nil {
+		t.Fatalf("Run: %v", err)
+	}
+
+	if rs.Status != runstore.StatusReadyForReview {
+		t.Errorf("want status %q (missing contract silently skipped), got %q",
+			runstore.StatusReadyForReview, rs.Status)
+	}
+	if rs.Cycle != 1 {
+		t.Errorf("want cycle 1 (no replan when contract file missing), got %d", rs.Cycle)
+	}
+	if rs.TotalReplans != 0 {
+		t.Errorf("expected 0 replans, got %d", rs.TotalReplans)
+	}
+}
diff --git a/internal/next/specloop/stages/write_contracts_test.go b/internal/next/specloop/stages/write_contracts_test.go
new file mode 100644
index 000000000..bced875d5
--- /dev/null
+++ b/internal/next/specloop/stages/write_contracts_test.go
@@ -0,0 +1,923 @@
+package stages
+
+import (
+	"context"
+	"fmt"
+	"os"
+	"path/filepath"
+	"strings"
+	"testing"
+
+	"github.com/danabrams/gromit/internal/next/contract"
+	"github.com/danabrams/gromit/internal/next/execpolicy"
+	"github.com/danabrams/gromit/internal/next/runstore"
+	"github.com/danabrams/gromit/internal/next/specloop"
+)
+
+// fakeContractWriter is a test double for the ContractWriter interface.
+type fakeContractWriter struct {
+	result *contract.ScenarioContract
+	err    error
+	calls  int
+}
+
+func (f *fakeContractWriter) WriteContracts(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+	f.calls++
+	if f.err != nil {
+		return nil, f.err
+	}
+	return f.result, nil
+}
+
+// Verify WriteContractsStage satisfies the Stage interface.
+var _ specloop.Stage = (*WriteContractsStage)(nil)
+
+func makeWriteContractsRunState(t *testing.T, store *runstore.Store) *runstore.RunState {
+	t.Helper()
+	rs := runstore.NewRunState("spec-001", "proj-001")
+	runDir := store.RunDir(rs.RunID)
+	if err := os.MkdirAll(runDir, 0o755); err != nil {
+		t.Fatalf("create run dir: %v", err)
+	}
+	return rs
+}
+
+const specWithScenarios = `# Test Spec
+
+## Scenarios
+
+### Scenario: add-works
+**When:** add is called with 1 and 2
+**Then:** result is 3
+
+### Scenario: subtract-works
+**When:** subtract is called with 5 and 3
+**Then:** result is 2
+`
+
+const specWithoutScenarios = `# Test Spec
+
+## Overview
+No scenarios here.
+`
+
+const specWithSkippedScenarios = `# Test Spec
+
+## Scenarios
+
+### Scenario: invalid-format
+This scenario has no proper format
+And should be skipped
+
+### Scenario: add-works
+**When:** add is called with 1 and 2
+**Then:** result is 3
+`
+
+func TestWriteContracts_IdempotencyNoOp(t *testing.T) {
+	// When ContractsWritten is already true, stage returns Continue without calling writer
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+	rs.ContractsWritten = true
+
+	writer := &fakeContractWriter{}
+	cfg := WriteContractsStageConfig{
+		SpecPath:    filepath.Join(tmp, "spec.md"),
+		EvidenceDir: filepath.Join(tmp, "evidence"),
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue for idempotent run, got %v", action.Kind)
+	}
+	if writer.calls != 0 {
+		t.Fatalf("expected 0 writer calls for idempotent run, got %d", writer.calls)
+	}
+}
+
+func TestWriteContracts_NoScenariosReturnsContinue(t *testing.T) {
+	// When spec has no scenarios, returns Continue with no output
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithoutScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	// Write spec-packet.md
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	writer := &fakeContractWriter{}
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue for no-scenarios, got %v", action.Kind)
+	}
+	if writer.calls != 0 {
+		t.Fatalf("expected 0 writer calls for no-scenarios, got %d", writer.calls)
+	}
+	// No contract file should be written
+	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
+	if _, err := os.Stat(contractPath); !os.IsNotExist(err) {
+		t.Fatal("expected no scenario-contracts.yaml for no-scenarios spec")
+	}
+}
+
+func TestWriteContracts_SuccessWritesContractFile(t *testing.T) {
+	// Happy path: writer returns valid contract, stage writes YAML and sets ContractsWritten=true
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{
+				Name: "add-works",
+				Assertions: []contract.ContractAssertion{
+					{FileExists: "calc/calc.go"},
+				},
+			},
+			{
+				Name: "subtract-works",
+				Assertions: []contract.ContractAssertion{
+					{FileExists: "calc/calc.go"},
+				},
+			},
+		},
+	}
+
+	writer := &fakeContractWriter{result: &validContract}
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+	if !rs.ContractsWritten {
+		t.Fatal("expected rs.ContractsWritten=true after success")
+	}
+
+	// Contract file must exist
+	contractPath := filepath.Join(evidenceDir, "scenario-contracts.yaml")
+	data, err := os.ReadFile(contractPath)
+	if err != nil {
+		t.Fatalf("scenario-contracts.yaml not written: %v", err)
+	}
+	if len(data) == 0 {
+		t.Fatal("scenario-contracts.yaml is empty")
+	}
+
+	// Event must be emitted
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+	var found bool
+	for _, ev := range events {
+		if ev.EventType() == "contracts_written" {
+			found = true
+		}
+	}
+	if !found {
+		t.Fatal("expected contracts_written event to be emitted")
+	}
+}
+
+func TestWriteContracts_WriterErrorRetriesAndBlocks(t *testing.T) {
+	// When writer always fails with parse/validation error, retries 3 total then returns Blocked
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// Writer returns invalid contract (zero fields per assertion)
+	invalidContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{
+				Name: "add-works",
+				Assertions: []contract.ContractAssertion{
+					{}, // zero fields — invalid
+				},
+			},
+		},
+	}
+
+	writer := &fakeContractWriter{result: &invalidContract}
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked after retry exhaustion, got %v", action.Kind)
+	}
+	// 3 total attempts (1 initial + 2 retries)
+	if writer.calls != 3 {
+		t.Fatalf("expected 3 writer calls (1+2 retries), got %d", writer.calls)
+	}
+
+	// contracts_blocked event must be emitted
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+	var found bool
+	for _, ev := range events {
+		if ev.EventType() == "contracts_blocked" {
+			found = true
+		}
+	}
+	if !found {
+		t.Fatal("expected contracts_blocked event to be emitted")
+	}
+}
+
+func TestWriteContracts_ValidationFailureRetriesOnce(t *testing.T) {
+	// First call returns invalid, second returns valid — only 2 calls total
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
+		},
+	}
+	invalidContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}},
+		},
+	}
+
+	callCount := 0
+	writer := &callbackContractWriter{
+		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+			callCount++
+			if callCount == 1 {
+				return &invalidContract, nil
+			}
+			return &validContract, nil
+		},
+	}
+
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue after retry success, got %v", action.Kind)
+	}
+	if callCount != 2 {
+		t.Fatalf("expected 2 writer calls, got %d", callCount)
+	}
+	if !rs.ContractsWritten {
+		t.Fatal("expected rs.ContractsWritten=true after retry success")
+	}
+}
+
+func TestWriteContracts_BudgetExhaustedReturnsBlocked(t *testing.T) {
+	// When budget is exhausted before LLM invocation, stage returns Blocked and emits contracts_blocked
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	budget := specloop.NewBudget(execpolicy.Budgets{MaxSpecCycles: 1, MaxRunCostUSD: 99, MaxRunDurationSeconds: 3600, MaxTaskDurationSeconds: 300})
+	budget.IncrementCycle() // exhaust the budget
+
+	writer := &fakeContractWriter{}
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, budget, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked when budget exhausted, got %v", action.Kind)
+	}
+	if writer.calls != 0 {
+		t.Fatalf("expected 0 writer calls when budget exhausted, got %d", writer.calls)
+	}
+
+	// contracts_blocked event must be emitted
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+	var found bool
+	for _, ev := range events {
+		if ev.EventType() == "contracts_blocked" {
+			found = true
+		}
+	}
+	if !found {
+		t.Fatal("expected contracts_blocked event when budget exhausted")
+	}
+}
+
+func TestWriteContracts_Name(t *testing.T) {
+	stage := &WriteContractsStage{}
+	if stage.Name() != "write_contracts" {
+		t.Fatalf("expected name 'write_contracts', got %q", stage.Name())
+	}
+}
+
+func TestWriteContracts_SkippedScenariosEmitEvents(t *testing.T) {
+	// When scenarios are skipped during parsing, events must be emitted
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithSkippedScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet content"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{
+				Name: "add-works",
+				Assertions: []contract.ContractAssertion{
+					{FileExists: "calc/calc.go"},
+				},
+			},
+		},
+	}
+
+	writer := &fakeContractWriter{result: &validContract}
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue, got %v", action.Kind)
+	}
+
+	// Events must be emitted for skipped scenarios
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+
+	var skippedCount int
+	for _, ev := range events {
+		if ev.EventType() == "contract_scenario_skipped" {
+			skippedCount++
+		}
+	}
+
+	if skippedCount == 0 {
+		t.Fatal("expected at least one contract_scenario_skipped event to be emitted")
+	}
+}
+
+func TestWriteContracts_RetryContextIncludesValidKeys(t *testing.T) {
+	// On vocabulary violation retry, specPacket must include valid assertion key names.
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
+		},
+	}
+	invalidContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}},
+		},
+	}
+
+	var specPacketOnRetry string
+	callCount := 0
+	writer := &callbackContractWriter{
+		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+			callCount++
+			if callCount == 1 {
+				return &invalidContract, nil
+			}
+			specPacketOnRetry = specPacket
+			return &validContract, nil
+		},
+	}
+
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue after retry success, got %v", action.Kind)
+	}
+
+	for _, key := range []string{"file_exists", "file_contains", "file_not_modified", "file_not_exists", "file_not_contains"} {
+		if !strings.Contains(specPacketOnRetry, key) {
+			t.Errorf("retry specPacket missing valid assertion key %q", key)
+		}
+	}
+}
+
+func TestWriteContracts_ContractsWrittenEventHasCount(t *testing.T) {
+	// The contracts_written event must carry the correct scenario count
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
+			{Name: "subtract-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
+		},
+	}
+	writer := &fakeContractWriter{result: &validContract}
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)
+
+	_, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+	for _, ev := range events {
+		if cwe, ok := ev.(*runstore.ContractsWrittenEvent); ok {
+			if cwe.ScenarioCount != 2 {
+				t.Fatalf("expected ScenarioCount=2, got %d", cwe.ScenarioCount)
+			}
+			return
+		}
+	}
+	t.Fatal("contracts_written event not found")
+}
+
+func TestWriteContracts_LLMErrorAppendsToRetryContext(t *testing.T) {
+	// On LLM/infrastructure error, the error message must appear in SpecContent on the next attempt.
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
+		},
+	}
+
+	const llmErrMsg = "rate limit exceeded: upstream overloaded"
+	var specPacketOnRetry string
+	callCount := 0
+	writer := &callbackContractWriter{
+		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+			callCount++
+			if callCount == 1 {
+				return nil, fmt.Errorf(llmErrMsg)
+			}
+			specPacketOnRetry = specPacket
+			return &validContract, nil
+		},
+	}
+
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue after retry success, got %v", action.Kind)
+	}
+	if callCount != 2 {
+		t.Fatalf("expected 2 writer calls, got %d", callCount)
+	}
+	if !strings.Contains(specPacketOnRetry, llmErrMsg) {
+		t.Errorf("retry specPacket missing LLM error message; got:\n%s", specPacketOnRetry)
+	}
+}
+
+// TestWriteContracts_IdempotentOnReplan verifies that the stage skips execution when
+// ContractsWritten is already true, which occurs after a replan cycle.
+func TestWriteContracts_IdempotentOnReplan(t *testing.T) {
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+	rs.ContractsWritten = true
+	rs.Cycle = 2 // simulate a replan cycle
+
+	writer := &fakeContractWriter{}
+	cfg := WriteContractsStageConfig{
+		SpecPath:    filepath.Join(tmp, "spec.md"),
+		EvidenceDir: filepath.Join(tmp, "evidence"),
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue for idempotent replan, got %v", action.Kind)
+	}
+	if writer.calls != 0 {
+		t.Fatalf("expected 0 writer calls on replan with ContractsWritten=true, got %d", writer.calls)
+	}
+}
+
+// TestWriteContracts_InvalidYAMLRetries verifies that when the contract writer returns
+// an error (e.g. invalid YAML from the LLM), the stage retries up to 3 total attempts
+// before returning Blocked.
+func TestWriteContracts_InvalidYAMLRetries(t *testing.T) {
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// Writer always returns a YAML parse error.
+	writer := &fakeContractWriter{err: fmt.Errorf("yaml: unmarshal error: cannot decode")}
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked after invalid YAML retries, got %v", action.Kind)
+	}
+	if writer.calls != 3 {
+		t.Fatalf("expected 3 writer calls (1+2 retries), got %d", writer.calls)
+	}
+
+	events, err := eventLog.ReadAll()
+	if err != nil {
+		t.Fatalf("read events: %v", err)
+	}
+	var found bool
+	for _, ev := range events {
+		if ev.EventType() == "contracts_blocked" {
+			found = true
+		}
+	}
+	if !found {
+		t.Fatal("expected contracts_blocked event after invalid YAML retries")
+	}
+}
+
+// TestWriteContracts_VocabularyViolation verifies that when the LLM produces assertions
+// with invalid keys (vocabulary violation), the retry prompt includes the list of valid
+// assertion key names so the LLM can self-correct.
+func TestWriteContracts_VocabularyViolation(t *testing.T) {
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// First call: vocabulary violation (zero fields = invalid assertion key usage).
+	// Second call: valid contract.
+	validContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{FileExists: "calc/calc.go"}}},
+		},
+	}
+	invalidContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}}, // no valid key set
+		},
+	}
+
+	var specPacketOnRetry string
+	callCount := 0
+	writer := &callbackContractWriter{
+		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+			callCount++
+			if callCount == 1 {
+				return &invalidContract, nil
+			}
+			specPacketOnRetry = specPacket
+			return &validContract, nil
+		},
+	}
+
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, nil)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Continue {
+		t.Fatalf("expected Continue after vocabulary violation retry, got %v", action.Kind)
+	}
+
+	// specPacket must include all valid assertion key names.
+	for _, key := range []string{"file_exists", "file_contains", "file_not_modified", "file_not_exists", "file_not_contains"} {
+		if !strings.Contains(specPacketOnRetry, key) {
+			t.Errorf("retry specPacket missing valid assertion key %q after vocabulary violation", key)
+		}
+	}
+}
+
+// TestWriteContracts_StaleValidationErrorsResetEachIteration verifies that validationErrors
+// is cleared at the top of each retry iteration so that a validation failure on attempt N
+// cannot leak into the terminal failure check when attempt N+1 returns an LLM error.
+func TestWriteContracts_StaleValidationErrorsResetEachIteration(t *testing.T) {
+	tmp := t.TempDir()
+	store := runstore.NewStore(tmp)
+	rs := makeWriteContractsRunState(t, store)
+
+	specPath := filepath.Join(tmp, "spec.md")
+	if err := os.WriteFile(specPath, []byte(specWithScenarios), 0o644); err != nil {
+		t.Fatal(err)
+	}
+	runDir := store.RunDir(rs.RunID)
+	if err := os.WriteFile(filepath.Join(runDir, "spec-packet.md"), []byte("packet"), 0o644); err != nil {
+		t.Fatal(err)
+	}
+
+	evidenceDir := filepath.Join(tmp, "evidence")
+	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
+		t.Fatal(err)
+	}
+
+	// Attempt 1: invalid contract (validation errors populated).
+	// Attempts 2 and 3: LLM error. After the loop lastErr != nil, validationErrors must be nil
+	// (not stale from attempt 1) so that the blocked reason correctly reflects the LLM error.
+	invalidContract := contract.ScenarioContract{
+		Scenarios: []contract.ScenarioAssertions{
+			{Name: "add-works", Assertions: []contract.ContractAssertion{{}}},
+		},
+	}
+	const llmErr = "upstream timeout"
+	callCount := 0
+	writer := &callbackContractWriter{
+		fn: func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+			callCount++
+			if callCount == 1 {
+				return &invalidContract, nil
+			}
+			return nil, fmt.Errorf(llmErr)
+		},
+	}
+
+	eventLogPath := filepath.Join(tmp, "events.jsonl")
+	eventLog := runstore.NewEventLog(eventLogPath)
+	cfg := WriteContractsStageConfig{
+		SpecPath:    specPath,
+		EvidenceDir: evidenceDir,
+		Store:       store,
+	}
+	stage := NewWriteContractsStage(writer, cfg, nil, eventLog)
+
+	action, err := stage.Run(context.Background(), rs)
+	if err != nil {
+		t.Fatalf("unexpected error: %v", err)
+	}
+	if action.Kind != specloop.Blocked {
+		t.Fatalf("expected Blocked, got %v", action.Kind)
+	}
+	// The blocked reason must reflect the LLM error, not stale validation errors from attempt 1.
+	if action.Context == nil || len(action.Context.Failures) == 0 {
+		t.Fatal("expected FailureContext with at least one failure")
+	}
+	reason := action.Context.Failures[0]
+	if !strings.Contains(reason, llmErr) {
+		t.Errorf("expected blocked reason to contain LLM error %q, got: %q", llmErr, reason)
+	}
+	if strings.Contains(reason, "validation failed") {
+		t.Errorf("blocked reason must not mention stale validation error, got: %q", reason)
+	}
+}
+
+// callbackContractWriter allows per-call behaviour in tests.
+type callbackContractWriter struct {
+	fn func(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error)
+}
+
+func (c *callbackContractWriter) WriteContracts(ctx context.Context, scenarios []contract.SpecScenario, specPacket string) (*contract.ScenarioContract, error) {
+	return c.fn(ctx, scenarios, specPacket)
+}
+

## Cycle History

| Cycle | Tasks | Passed |
|-------|-------|--------|
| 1 | 66 | 55 |

## Validation Results

pass=true

## Known Risks


## Review Findings

| Facet | Count | Severities |
|-------|-------|------------|
| code_quality | 8 | 1 info, 5 suggestion, 2 warning |
| spec_alignment | 6 | 2 info, 2 suggestion, 2 warning |

## Acceptance Criteria

| Criterion | Status | Rationale |
|-----------|--------|-----------|
| A WriteContracts stage runs after Plan and before Execute, producing a `scenario-contracts.yaml` file in the run's evidence directory | pass | The evidence clearly shows: (1) `WriteContractsStage` is implemented in `internal/next/specloop/stages/write_contracts.go` with `Name() = "write_contracts"`, (2) it is inserted into the stage pipeline in `stage_provider.go` after `planStage` and before `executeStage`, (3) it writes `scenario-contracts.yaml` to `EvidenceDir` on success, and (4) integration tests confirm the file is created in the evidence directory. |
| The WriteContracts stage translates each spec Scenario into declarative assertions using only the in-pipeline assertion vocabulary (`file_exists`, `file_contains`, `file_not_modified`, `file_not_exists`, `file_not_contains`) | pass | The implementation is clearly satisfied. The ContractAssertion type in types.go defines exactly these five fields. The prompt.txt explicitly instructs the LLM to use only these five assertion types. ValidateContract() in validate.go enforces vocabulary compliance by checking exactly one of these five fields is set per assertion. The retry loop in write_contracts.go feeds validation errors back to the LLM with the valid key names when violations occur. Tests in validate_test.go and write_contracts_test.go verify all five assertion types round-trip correctly and vocabulary violations trigger retries. |
| Generated contract assertions are validated against the known vocabulary; unknown assertion keys trigger a retry, then `blocked` | pass | The implementation fully satisfies the criterion. `contract.ValidateContract` in `validate.go` checks each `ContractAssertion` has exactly one of the five known fields set (file_exists, file_contains, file_not_modified, file_not_exists, file_not_contains). In `write_contracts.go`, the retry loop calls `ValidateContract` after each LLM invocation; vocabulary violations prepend the errors plus 'Valid assertion keys: ...' to the specPacket for the next attempt. After 3 total attempts with persistent validation errors, the stage returns `Blocked`. Tests `TestWriteContracts_VocabularyViolation`, `TestWriteContracts_RetryContextIncludesValidKeys`, `TestWriteContracts_WriterErrorRetriesAndBlocks`, and `TestWriteContracts_ValidationFailureRetriesOnce` directly verify this behavior. |
| All scenarios are processed in a single LLM invocation during contract writing (batch, not sequential) | pass | LLMContractWriter.WriteContracts receives the full []SpecScenario slice, renders all scenarios into a single prompt via the template ({{range .Scenarios}} in prompt.txt), and makes exactly one w.invoker.Invoke(ctx, prompt) call. There is no per-scenario loop around the invoke call. The retry loop in WriteContractsStage re-invokes once per retry attempt (not per scenario), still passing the full scenario list each time. |
| The Validate stage checks contract assertions via an injected `ContractEvaluator` interface (not a `validator.Check`), using `EvidenceDir` from `ValidateStageConfig` to locate the contract file | pass | ValidateStageConfig gains an `EvidenceDir string` field. ValidateStage stores a `contractEvaluator contract.ContractEvaluator` (injected via NewValidateStage's new evaluator parameter). In Run(), it joins `s.cfg.EvidenceDir` with `scenario-contracts.yaml` to locate the contract file, parses it, then calls `s.contractEvaluator.Evaluate(ctx, &sc, workDir)` — not a validator.Check. The ContractEvaluator interface is defined in `internal/next/contract/evaluator.go` and is separate from the validator package. |
| Contract assertion failures produce failure context identifying the scenario name and failed assertion | pass | In validate.go, contract failures are formatted as `fmt.Sprintf("contract:%s — %s failed: %s", f.ScenarioName, f.AssertionType, f.Details)`, embedding both the scenario name and assertion type. TestValidateStage_ContractFailures verifies this exact format: `contract:subtract-works — file_exists failed: file "result.txt" does not exist`. |
| Contract failures trigger replan via the existing `replan_from` mechanism | pass | ValidateStage now collects contract assertion failures from scenario-contracts.yaml and appends them to the failures slice before computing finalPassed. When contract failures exist, finalPassed=false and the stage returns NextAction{Kind: ReplanFrom} with the failure context — exactly the existing replan_from mechanism. This is directly tested in TestValidateStage_ContractFailures (unit) and TestIntegration_ContractFailureTriggersReplan_ReplanStageBypassesWriteContracts (integration). |
| On replan cycles, WriteContracts is a no-op when its RunState flag (`ContractsWritten`) is true — fix tasks target the implementation, not the contracts. WriteContracts sets `rs.ContractsWritten = true` on success. | pass | All three parts of the criterion are satisfied: (1) WriteContractsStage.Run() opens with `if rs.ContractsWritten { return specloop.NextAction{Kind: specloop.Continue}, nil }` — the idempotency guard; (2) on success the stage sets `rs.ContractsWritten = true`; (3) `runstore.ResetForNewCycle` explicitly omits ContractsWritten from the per-cycle reset, preserving it across replan cycles. Multiple unit and integration tests verify the no-op behavior: TestWriteContracts_IdempotencyNoOp, TestWriteContracts_IdempotentOnReplan, TestIntegration_WriteContractsIdempotentOnReplan, TestIntegration_WriteContractsIdempotentOnReplanFromPlan, and TestContractsWritten_PersistedAcrossReplanCycles. |
| If WriteContracts produces unparseable YAML or invalid assertions after two retries, the stage returns `blocked` | pass | write_contracts.go implements a retry loop with maxAttempts=3 (1 initial + 2 retries). After exhaustion, if lastErr != nil or len(validationErrors) > 0, it returns specloop.NextAction{Kind: specloop.Blocked}. Two tests directly verify this: TestWriteContracts_WriterErrorRetriesAndBlocks (invalid assertions, expects 3 calls then Blocked) and TestWriteContracts_InvalidYAMLRetries (YAML parse error always returned, expects 3 calls then Blocked). |
| If the spec has no Scenarios section or it is empty, WriteContracts is a no-op (returns `Continue`) and Validate skips contract checking | pass | WriteContractsStage.Run() calls contract.ParseScenarios() and immediately returns Continue when len(scenarios) == 0 (write_contracts.go, early return after parsing). Since no contract file is written, ValidateStage skips contract evaluation because os.ReadFile returns ErrNotExist which is handled silently (validate.go: `if err != nil && !errors.Is(err, os.ErrNotExist)`). Both behaviors are verified by integration tests: TestIntegration_NoScenariosWriteContractsNoOp confirms writer.calls==0, no contract file written, pipeline completes without replan; TestValidateStage_MissingContractFile and TestIntegration_MissingContractFileSkippedSilently confirm validate proceeds without error when contract file is absent. |
| If `scenario-contracts.yaml` does not exist in `EvidenceDir` at Validate time, contract checking is skipped silently (not a failure) | pass | The implementation in validate.go explicitly checks for os.ErrNotExist and silently skips contract evaluation — only non-ErrNotExist errors propagate as failures. This is tested by TestValidateStage_MissingContractFile (unit), TestIntegration_MissingContractFileSkippedSilently (specloop integration), and TestIntegration_WriteContracts_MissingContractFileGraceful (stages integration), all verifying that a missing file yields Continue/ReadyForReview with no replans. |
| The WriteContracts stage uses an injected `ContractWriter` interface matching the existing `PlanCreator`/`ReviewRunner` pattern for testability | pass | The `ContractWriter` interface is defined in `internal/next/contract/llm_writer.go` and injected into `WriteContractsStage` via its constructor (`NewWriteContractsStage(writer contract.ContractWriter, ...)`). This mirrors the `PlanCreator`/`ReviewRunner` pattern exactly: a real LLM-backed implementation (`LLMContractWriter`), a noop implementation (`noopContractWriter`) for the no-provider path in `stage_provider.go`, and `fakeContractWriter`/`callbackContractWriter` test doubles used throughout the unit and integration tests. |
| `BuildStages` in `cmd/gromit-next/stage_provider.go` is updated to include WriteContracts in the correct pipeline position | pass | The diff shows `writeContractsStage` added to the returned stages slice at line 267 in `stage_provider.go`, positioned after `planStage` and before `executeStage`. Tests in `stage_provider_test.go` verify the expected stage order is `["init", "compile", "plan", "write_contracts", "execute", "validate", "review", "accept", "evidence", "finalize"]` (10 stages total), and the stage count assertions are updated from 9 to 10. |
| `ContractsWritten` RunState flag is NOT reset in the per-cycle reset block in `specloop.go` | pass | The per-cycle reset was refactored to call `runstore.ResetForNewCycle(rs)` in specloop.go. That function explicitly does NOT reset `ContractsWritten` — the comment in store.go states 'ContractsWritten is NOT reset — contracts are written once and persist across cycles.' Additionally, `TestContractsWritten_PersistedAcrossReplanCycles` in specloop_test.go directly verifies this behavior. |
| WriteContracts uses Sonnet (P1) model tier | pass | In stage_provider.go, the contract writer adapter is created with `llmadapter.NewFallbackAdapter(router, "contracts", llmadapter.Config{Tier: policy.Models.Planner, ...}, policy.Models.Planner)`. The Planner tier maps to Sonnet (P1), consistent with the codebase's model tier conventions. The LLMContractWriter comment also explicitly states 'Uses Sonnet (P1) model tier'. The review finding notes there's no dedicated test for this wiring, but the implementation itself clearly uses the correct tier. |
| All existing pipeline tests continue to pass | pass | The validation results explicitly show pass=true. The diff shows existing test files were updated to match the new stage signatures (e.g., NewValidateStage now takes a ContractEvaluator parameter, stage counts updated from 9 to 10) rather than breaking existing tests. The test suite ran successfully with the updated signatures. |
| The `SpecScenario` type and `ParseScenarios` function are exported from `internal/next/contract/` for reuse by downstream specs (specifically 0002f) | pass | `SpecScenario` is defined as an exported struct in `internal/next/contract/types.go` and `ParseScenarios` is defined as an exported function in `internal/next/contract/parse.go`. Both use uppercase names making them accessible to downstream packages. They are also used across package boundaries (e.g., in `internal/next/specloop/stages/write_contracts.go` which imports and calls `contract.ParseScenarios`). |
| WriteContracts emits events: `contracts_written` on success (with scenario count), `contracts_blocked` on terminal failure | pass | The implementation clearly satisfies both parts of the criterion. In write_contracts.go, a `ContractsWrittenEvent` is emitted on success with `ScenarioCount: len(result.Scenarios)`, and a `ContractsBlockedEvent` is emitted on both terminal failure paths (budget exhaustion and retry exhaustion after 3 attempts). Both event types are defined in events.go with proper JSON marshaling and unmarshal support. Tests explicitly verify: `TestWriteContracts_ContractsWrittenEventHasCount` confirms ScenarioCount=2 is correct, `TestWriteContracts_SuccessWritesContractFile` confirms the `contracts_written` event is emitted, and `TestWriteContracts_WriterErrorRetriesAndBlocks` / `TestWriteContracts_BudgetExhaustedReturnsBlocked` / `TestWriteContracts_InvalidYAMLRetries` all verify `contracts_blocked` is emitted on terminal failure. |
| The in-pipeline assertion vocabulary includes `file_not_exists` and `file_not_contains` in addition to `file_exists`, `file_contains`, and `file_not_modified` | pass | All five assertion types are present throughout the implementation: ContractAssertion struct defines FileNotExists and FileNotContains fields (types.go), DefaultContractEvaluator.check() handles both cases with correct semantics (evaluator.go), ValidateContract counts all five fields (validate.go), the LLM prompt explicitly documents all five assertion types (prompt.txt), and dedicated passing/failing tests exist for both file_not_exists and file_not_contains (evaluator_test.go, parse_test.go). |
| `file_contains` and `file_not_contains` use literal substring matching (`strings.Contains`), matching the E2E harness behavior | pass | Both assertions use `strings.Contains` directly in `DefaultContractEvaluator.check`. The `FileContainsAssertion.Pattern` field is also documented with the comment `// Literal substring, matched via strings.Contains` in types.go. |
| Contract failure messages use the format `contract:<scenario-name> — <assertion-type> failed: <details>` — this format is a shared contract with spec 0002f's FailureHistory key extraction | pass | The format is implemented exactly in validate.go with `fmt.Sprintf("contract:%s — %s failed: %s", f.ScenarioName, f.AssertionType, f.Details)`, and a unit test in validate_test.go asserts the exact string `contract:subtract-works — file_exists failed: file "result.txt" does not exist`, confirming both implementation and format contract are in place. |

## Recommended Action

review
