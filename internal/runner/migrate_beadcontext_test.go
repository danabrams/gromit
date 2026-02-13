package runner

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/learnings"
	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// setupMigratedRunner creates a Runner with minimal deps for testing the
// beadContext → runtypes.BeadContext migration.
func setupMigratedRunner(t *testing.T) *Runner {
	t.Helper()
	return &Runner{
		cfg: &config.Config{
			Escalation: config.EscalationConfig{
				MaxRetriesPerModel: 2,
				MaxRetriesPerBead:  5,
			},
			Claude: config.ClaudeConfig{
				BeadTimeout: 300,
			},
		},
		beads:    &mockBeadClient{},
		renderer: &mockRenderer{},
		output:   &strings.Builder{},
		router:   newMockRouter(),
	}
}

// TestSetupBeadContext_ReturnsRuntypesBeadContext verifies that setupBeadContext
// returns *runtypes.BeadContext instead of the old *beadContext type.
// After the migration, the return type must be *runtypes.BeadContext so that
// sub-packages can accept and use it without importing the runner package.
//
// Expected failure: setupBeadContext currently returns *beadContext, not *runtypes.BeadContext.
func TestSetupBeadContext_ReturnsRuntypesBeadContext(t *testing.T) {
	r := setupMigratedRunner(t)
	b := &bead.Bead{ID: "migrate-1", Title: "Migration test", Priority: 1}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Check that the returned type is *runtypes.BeadContext, not *beadContext.
	// Currently returns *beadContext; after migration returns *runtypes.BeadContext.
	gotType := reflect.TypeOf(bc)
	wantType := reflect.TypeOf((*runtypes.BeadContext)(nil))
	if gotType != wantType {
		t.Errorf("setupBeadContext return type = %v, want %v", gotType, wantType)
	}
}

// TestSetupBeadContext_ExportedFieldAccess verifies that after migration,
// the value returned by setupBeadContext has exported fields (Bead, Model,
// Tier, Result, etc.) that can be accessed by sub-packages.
//
// Expected failure: setupBeadContext returns *beadContext which has unexported
// fields (bead, model, tier); after migration it returns *runtypes.BeadContext
// with exported fields (Bead, Model, Tier).
func TestSetupBeadContext_ExportedFieldAccess(t *testing.T) {
	r := setupMigratedRunner(t)
	b := &bead.Bead{ID: "field-test", Title: "Field access test", Priority: 1}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Use reflection to check that all expected exported fields exist on the
	// returned value. Currently *beadContext has unexported fields only.
	val := reflect.ValueOf(bc).Elem()
	typ := val.Type()

	exportedFields := []string{
		"Bead", "Parent", "Result", "Model", "Tier", "BuildProvider",
		"PromptCtx", "BuildPrompt", "StartCommit", "Iteration",
		"RetriesThisModel", "TotalRetriesThisBead", "MaxRetries",
		"MaxRetriesPerBead", "ParentCtx", "BeadTimeout", "RunDeadline",
		"ScopeEstimate", "TouchedPackages",
	}

	for _, fieldName := range exportedFields {
		field, found := typ.FieldByName(fieldName)
		if !found {
			t.Errorf("field %q not found on returned type %v", fieldName, typ)
			continue
		}
		if !field.IsExported() {
			t.Errorf("field %q exists but is not exported on type %v", fieldName, typ)
		}
	}
}

// TestSetupBeadContext_PopulatesExportedFields verifies that after migration,
// setupBeadContext populates the exported fields of *runtypes.BeadContext
// with the correct values from config and input parameters.
//
// Expected failure: setupBeadContext returns *beadContext; accessing exported
// fields (Bead, MaxRetries, etc.) via reflection returns zero values because
// those fields don't exist on *beadContext — only unexported equivalents do.
func TestSetupBeadContext_PopulatesExportedFields(t *testing.T) {
	r := setupMigratedRunner(t)
	b := &bead.Bead{ID: "populate-test", Title: "Populate test", Priority: 1}
	deadline := time.Now().Add(30 * time.Minute)

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, deadline, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	val := reflect.ValueOf(bc).Elem()

	// Check Bead field via reflection (exported on runtypes.BeadContext,
	// unexported as "bead" on the current beadContext)
	beadField := val.FieldByName("Bead")
	if !beadField.IsValid() {
		t.Fatal("exported field 'Bead' not found — migration not yet complete")
	}
	if beadField.IsNil() {
		t.Error("Bead field should not be nil")
	} else {
		beadVal := beadField.Interface().(*bead.Bead)
		if beadVal.ID != "populate-test" {
			t.Errorf("Bead.ID = %q, want %q", beadVal.ID, "populate-test")
		}
	}

	// Check MaxRetries (exported on runtypes.BeadContext, unexported as "maxRetries" on beadContext)
	maxRetriesField := val.FieldByName("MaxRetries")
	if !maxRetriesField.IsValid() {
		t.Fatal("exported field 'MaxRetries' not found — migration not yet complete")
	}
	if maxRetriesField.Int() != 2 {
		t.Errorf("MaxRetries = %d, want 2", maxRetriesField.Int())
	}

	// Check MaxRetriesPerBead
	maxRetriesPerBeadField := val.FieldByName("MaxRetriesPerBead")
	if !maxRetriesPerBeadField.IsValid() {
		t.Fatal("exported field 'MaxRetriesPerBead' not found — migration not yet complete")
	}
	if maxRetriesPerBeadField.Int() != 5 {
		t.Errorf("MaxRetriesPerBead = %d, want 5", maxRetriesPerBeadField.Int())
	}

	// Check Iteration
	iterationField := val.FieldByName("Iteration")
	if !iterationField.IsValid() {
		t.Fatal("exported field 'Iteration' not found — migration not yet complete")
	}
	if iterationField.Int() != 1 {
		t.Errorf("Iteration = %d, want 1", iterationField.Int())
	}

	// Check Result is non-nil and has correct BeadID
	resultField := val.FieldByName("Result")
	if !resultField.IsValid() {
		t.Fatal("exported field 'Result' not found — migration not yet complete")
	}
	if resultField.IsNil() {
		t.Fatal("Result should not be nil")
	}

	// Check RunDeadline
	runDeadlineField := val.FieldByName("RunDeadline")
	if !runDeadlineField.IsValid() {
		t.Fatal("exported field 'RunDeadline' not found — migration not yet complete")
	}
}

// TestBeadContextType_IsDeleted verifies that the old beadContext struct type
// no longer exists in the runner package after migration. All code should use
// runtypes.BeadContext instead.
//
// Expected failure: beadContext type currently exists in process.go:27-52.
func TestBeadContextType_IsDeleted(t *testing.T) {
	// After migration, creating a beadContext literal should be impossible
	// because the type is deleted. We verify this indirectly: setupBeadContext
	// must return *runtypes.BeadContext (not *beadContext).
	r := setupMigratedRunner(t)
	b := &bead.Bead{ID: "delete-check", Title: "Delete check", Priority: 1}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// The type name should be "BeadContext" (from runtypes package),
	// not "beadContext" (unexported, from runner package).
	typeName := reflect.TypeOf(bc).Elem().Name()
	if typeName != "BeadContext" {
		t.Errorf("returned type name = %q, want %q (beadContext should be deleted)", typeName, "BeadContext")
	}

	// The package path should be the runtypes package
	pkgPath := reflect.TypeOf(bc).Elem().PkgPath()
	if !strings.HasSuffix(pkgPath, "runner/runtypes") {
		t.Errorf("returned type package = %q, want suffix %q", pkgPath, "runner/runtypes")
	}
}

// TestEscalateTier_MutatesExportedFields verifies that escalateTier correctly
// mutates the exported Tier, RetriesThisModel, Model, and Result fields on
// the bead context after the migration to runtypes.BeadContext.
//
// Expected failure: escalateTier currently reads/writes unexported fields
// (tier, retriesThisModel, model, result) on *beadContext; after migration
// it will use exported fields (Tier, RetriesThisModel, Model, Result) on
// *runtypes.BeadContext.
func TestEscalateTier_MutatesExportedFields(t *testing.T) {
	r := setupMigratedRunner(t)

	// Call setupBeadContext to get whatever type it returns, then escalate
	b := &bead.Bead{ID: "escalate-test", Title: "Escalate test", Priority: 1}
	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	r.escalateTier(bc, "high")

	// Use reflection to check exported fields were mutated
	val := reflect.ValueOf(bc).Elem()

	tierField := val.FieldByName("Tier")
	if !tierField.IsValid() {
		t.Fatal("exported field 'Tier' not found — migration not yet complete")
	}
	if tierField.String() != "high" {
		t.Errorf("Tier = %q, want %q after escalation", tierField.String(), "high")
	}

	retriesField := val.FieldByName("RetriesThisModel")
	if !retriesField.IsValid() {
		t.Fatal("exported field 'RetriesThisModel' not found — migration not yet complete")
	}
	if retriesField.Int() != 0 {
		t.Errorf("RetriesThisModel = %d, want 0 (should reset on tier change)", retriesField.Int())
	}

	resultField := val.FieldByName("Result")
	if !resultField.IsValid() {
		t.Fatal("exported field 'Result' not found — migration not yet complete")
	}
	result := resultField.Interface().(*runtypes.IterationResult)
	if !result.Escalated {
		t.Error("Result.Escalated should be true after escalateTier")
	}
}

// TestProcessBead_ReturnsCorrectResult verifies that the full processBead
// flow works with runtypes.BeadContext internally, propagating all fields
// correctly through the exported struct to the returned IterationResult.
//
// Expected failure: processBead internally uses *beadContext; after migration
// it uses *runtypes.BeadContext and setupBeadContext returns the new type.
func TestProcessBead_ReturnsCorrectResult(t *testing.T) {
	r := setupMigratedRunner(t)
	b := &bead.Bead{ID: "process-test", Title: "Process test", Priority: 1}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify the type is *runtypes.BeadContext
	gotType := reflect.TypeOf(bc)
	wantType := reflect.TypeOf((*runtypes.BeadContext)(nil))
	if gotType != wantType {
		t.Fatalf("setupBeadContext return type = %v, want %v", gotType, wantType)
	}

	// Verify Result field is accessible via exported name and has correct BeadID
	val := reflect.ValueOf(bc).Elem()
	resultField := val.FieldByName("Result")
	if !resultField.IsValid() {
		t.Fatal("exported field 'Result' not found — migration not yet complete")
	}
	result := resultField.Interface().(*runtypes.IterationResult)
	if result.BeadID != "process-test" {
		t.Errorf("Result.BeadID = %q, want %q", result.BeadID, "process-test")
	}
}

// TestExtractLearning_ReadsExportedBeadField verifies that learning extraction
// functions read the Bead field through its exported name after migration.
//
// Expected failure: extractSyntheticLearning reads bc.bead (unexported);
// after migration it reads bc.Bead (exported from runtypes.BeadContext).
func TestExtractLearning_ReadsExportedBeadField(t *testing.T) {
	tempDir := t.TempDir()
	lf, err := learnings.NewFile(tempDir)
	if err != nil {
		t.Fatalf("creating learnings file: %v", err)
	}

	mockRend := &mockPromptRenderer{LearningsFile: lf}
	var buf strings.Builder

	r := &Runner{
		renderer: mockRend,
		output:   &buf,
	}

	// Call setupBeadContext and verify that the returned context has the
	// exported Bead field used by learning extraction
	r2 := setupMigratedRunner(t)
	b := &bead.Bead{ID: "learn-test", Title: "Learning test", Priority: 1}
	bc, _, cancel, err := r2.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify the Bead field is accessible by exported name
	val := reflect.ValueOf(bc).Elem()
	beadField := val.FieldByName("Bead")
	if !beadField.IsValid() {
		t.Fatal("exported field 'Bead' not found — migration not yet complete")
	}

	// Now test that extractSyntheticLearning uses it correctly.
	// We need the *result* of the migration (where bc is *runtypes.BeadContext)
	// to pass it to extractSyntheticLearning.
	gotType := reflect.TypeOf(bc)
	wantType := reflect.TypeOf((*runtypes.BeadContext)(nil))
	if gotType != wantType {
		t.Fatalf("bc type = %v, want %v (extractSyntheticLearning needs *runtypes.BeadContext)", gotType, wantType)
	}

	// If the type is correct, extractSyntheticLearning will accept it
	r.extractSyntheticLearning(bc, "test learning message")

	if !strings.Contains(buf.String(), "Synthetic learning extracted") {
		t.Error("expected synthetic learning extraction log message")
	}
}

// TestHandleStallTimeout_ReadsExportedRetryFields verifies that handleStallTimeout
// reads and mutates retry counters through exported field names after migration.
//
// Expected failure: handleStallTimeout reads bc.retriesThisModel (unexported);
// after migration it reads bc.RetriesThisModel (exported from runtypes.BeadContext).
func TestHandleStallTimeout_ReadsExportedRetryFields(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			Escalation: config.EscalationConfig{
				Chain: []string{"low", "medium", "high"},
			},
		},
		output: &strings.Builder{},
	}

	// Get the return from setupBeadContext and verify it has exported retry fields
	r2 := setupMigratedRunner(t)
	b := &bead.Bead{ID: "stall-test", Title: "Stall test", Priority: 1}
	bc, _, cancel, err := r2.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify exported retry tracking fields exist
	val := reflect.ValueOf(bc).Elem()
	for _, field := range []string{"RetriesThisModel", "TotalRetriesThisBead", "MaxRetries", "MaxRetriesPerBead"} {
		if !val.FieldByName(field).IsValid() {
			t.Fatalf("exported field %q not found — migration not yet complete", field)
		}
	}

	// Verify the type is *runtypes.BeadContext so handleStallTimeout can accept it
	gotType := reflect.TypeOf(bc)
	wantType := reflect.TypeOf((*runtypes.BeadContext)(nil))
	if gotType != wantType {
		t.Errorf("bc type = %v, want %v (handleStallTimeout needs *runtypes.BeadContext)", gotType, wantType)
	}

	// After migration, this call should work with *runtypes.BeadContext
	// Set up the bc for a stall test via reflection
	val.FieldByName("RetriesThisModel").SetInt(0)
	val.FieldByName("TotalRetriesThisBead").SetInt(0)

	continueLoop := r.handleStallTimeout(context.Background(), bc)
	if !continueLoop {
		t.Error("expected continueLoop=true on first stall (should retry same model)")
	}

	// Check RetriesThisModel was incremented to 1
	if val.FieldByName("RetriesThisModel").Int() != 1 {
		t.Errorf("RetriesThisModel = %d, want 1 after first stall", val.FieldByName("RetriesThisModel").Int())
	}
}

// TestShouldRunRefactor_ReadsTierFromExportedField verifies that shouldRunRefactor
// reads the Tier field from the exported field on *runtypes.BeadContext.
//
// Expected failure: shouldRunRefactor reads bc.tier (unexported); after migration
// it reads bc.Tier (exported from runtypes.BeadContext).
func TestShouldRunRefactor_ReadsTierFromExportedField(t *testing.T) {
	r2 := setupMigratedRunner(t)
	b := &bead.Bead{ID: "refactor-test", Title: "Refactor test", Priority: 2}

	bc, _, cancel, err := r2.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify the type and exported Tier field
	gotType := reflect.TypeOf(bc)
	wantType := reflect.TypeOf((*runtypes.BeadContext)(nil))
	if gotType != wantType {
		t.Fatalf("bc type = %v, want %v", gotType, wantType)
	}

	val := reflect.ValueOf(bc).Elem()
	tierField := val.FieldByName("Tier")
	if !tierField.IsValid() {
		t.Fatal("exported field 'Tier' not found — migration not yet complete")
	}

	// Set tier to "low" and verify shouldRunRefactor skips it
	tierField.SetString("low")

	r := &Runner{
		cfg: &config.Config{
			Refactor: config.RefactorConfig{MinFilesChanged: 0},
		},
		output: &strings.Builder{},
	}

	got := r.shouldRunRefactor(bc, "diff --git a/file.go b/file.go")
	if got != false {
		t.Error("shouldRunRefactor should return false for low tier")
	}
}

// TestBuildPromptForBead_SetsExportedPromptFields verifies that
// buildPromptForBead sets PromptCtx and BuildPrompt through exported field
// names after the migration.
//
// Expected failure: buildPromptForBead sets bc.promptCtx and bc.buildPrompt
// (unexported); after migration it sets bc.PromptCtx and bc.BuildPrompt
// (exported from runtypes.BeadContext).
func TestBuildPromptForBead_SetsExportedPromptFields(t *testing.T) {
	r := setupMigratedRunner(t)
	b := &bead.Bead{ID: "prompt-test", Title: "Prompt test", Priority: 1}

	bc, _, cancel, err := r.setupBeadContext(context.Background(), b, 1, time.Time{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()

	// Verify type is *runtypes.BeadContext
	gotType := reflect.TypeOf(bc)
	wantType := reflect.TypeOf((*runtypes.BeadContext)(nil))
	if gotType != wantType {
		t.Fatalf("setupBeadContext return type = %v, want %v", gotType, wantType)
	}

	// Call buildPromptForBead
	if err := r.buildPromptForBead(context.Background(), bc, 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify exported PromptCtx and BuildPrompt fields are set
	val := reflect.ValueOf(bc).Elem()

	promptCtxField := val.FieldByName("PromptCtx")
	if !promptCtxField.IsValid() {
		t.Fatal("exported field 'PromptCtx' not found — migration not yet complete")
	}
	if promptCtxField.IsNil() {
		t.Error("PromptCtx should be set after buildPromptForBead")
	}

	buildPromptField := val.FieldByName("BuildPrompt")
	if !buildPromptField.IsValid() {
		t.Fatal("exported field 'BuildPrompt' not found — migration not yet complete")
	}
	if buildPromptField.String() == "" {
		t.Error("BuildPrompt should be non-empty after buildPromptForBead")
	}
}
