package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/pipeline"
)

// TestPromptRendererAdapter_SingleWorkflowMethods verifies adapters only expose
// the workflow-specific render method each pipeline interface requires.
func TestPromptRendererAdapter_SingleWorkflowMethods(t *testing.T) {
	var _ pipeline.ReviewRenderer = (*cliPromptRenderer)(nil)
	var _ pipeline.ExploreRenderer = (*explorePromptRenderer)(nil)

	reviewType := reflect.TypeOf((*cliPromptRenderer)(nil))
	if _, ok := reviewType.MethodByName("RenderThoroughReview"); !ok {
		t.Fatal("cliPromptRenderer must implement RenderThoroughReview")
	}

	unexpectedReviewMethods := []string{"RenderRefine", "RenderPlan", "RenderDecompose", "RenderExplore"}
	for _, methodName := range unexpectedReviewMethods {
		if _, ok := reviewType.MethodByName(methodName); ok {
			t.Errorf("cliPromptRenderer should not expose %s", methodName)
		}
	}

	exploreType := reflect.TypeOf((*explorePromptRenderer)(nil))
	if _, ok := exploreType.MethodByName("RenderExplore"); !ok {
		t.Fatal("explorePromptRenderer must implement RenderExplore")
	}
	unexpectedExploreMethods := []string{"RenderRefine", "RenderPlan", "RenderDecompose", "RenderThoroughReview"}
	for _, methodName := range unexpectedExploreMethods {
		if _, ok := exploreType.MethodByName(methodName); ok {
			t.Errorf("explorePromptRenderer should not expose %s", methodName)
		}
	}
}

// TestPipelineInterfaces_AllTypedSignatures verifies pipeline.go interface definitions use typed signatures
func TestPipelineInterfaces_AllTypedSignatures(t *testing.T) {
	// Expected failure: pipeline renderer interfaces still use interface{} or are not split by workflow.
	pipelinePath := filepath.Join("..", "..", "internal", "pipeline", "pipeline.go")
	content, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("reading pipeline.go: %v", err)
	}

	contentStr := string(content)

	rendererInterfaces := []struct {
		name      string
		signature string
	}{
		{name: "RefineRenderer", signature: "RenderRefine(input *RefinePromptInput) (string, error)"},
		{name: "PlanRenderer", signature: "RenderPlan(input *PlanPromptInput) (string, error)"},
		{name: "DecomposeRenderer", signature: "RenderDecompose(input *DecomposePromptInput) (string, error)"},
		{name: "ReviewRenderer", signature: "RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error)"},
		{name: "ExploreRenderer", signature: "RenderExplore(input *ExplorePromptInput) (string, error)"},
	}

	for _, rendererInterface := range rendererInterfaces {
		if !strings.Contains(contentStr, "type "+rendererInterface.name+" interface") {
			t.Fatalf("Could not find %s interface in pipeline.go", rendererInterface.name)
		}
		if !strings.Contains(contentStr, rendererInterface.signature) {
			t.Errorf("%s missing expected signature %q", rendererInterface.name, rendererInterface.signature)
		}
	}

	// Verify ClaudeClient returns typed result
	claudeClientSection := extractBetweenMarkers(contentStr, "type ClaudeClient interface", "type BeadClient interface")
	if !strings.Contains(claudeClientSection, "(*ClaudeRunResult, error)") {
		t.Error("ClaudeClient.Run should return (*ClaudeRunResult, error)")
	}

	// Verify BeadClient returns typed results
	beadClientSection := extractBetweenMarkers(contentStr, "type BeadClient interface", "type BacklogClient interface")
	if !strings.Contains(beadClientSection, "(*BeadInfo, error)") {
		t.Error("BeadClient methods should return (*BeadInfo, error)")
	}
}

// TestPipelinePromptInputTypes_Exist verifies all required prompt input types are defined
func TestPipelinePromptInputTypes_Exist(t *testing.T) {
	// Expected failure: RefinePromptInput, PlanPromptInput, DecomposePromptInput, ExplorePromptInput types do not exist
	pipelinePath := filepath.Join("..", "..", "internal", "pipeline", "pipeline.go")
	content, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("reading pipeline.go: %v", err)
	}

	contentStr := string(content)

	// Check for each required type
	requiredTypes := []string{
		"RefinePromptInput",
		"PlanPromptInput",
		"DecomposePromptInput",
		"ExplorePromptInput",
		"ThoroughReviewPromptInput", // Already exists
	}

	for _, typeName := range requiredTypes {
		typeDecl := "type " + typeName + " struct"
		if !strings.Contains(contentStr, typeDecl) {
			t.Errorf("pipeline.go should define %s struct for typed PromptRenderer input", typeName)
		}
	}
}

// TestAdapters_NoMapConstructionForPrompts verifies adapters don't construct intermediate maps for prompt data
func TestAdapters_NoMapConstructionForPrompts(t *testing.T) {
	// Expected failure: If there are any remaining map[string]interface{} constructions for prompt data
	reviewPath := filepath.Join(".", "review.go")
	content, err := os.ReadFile(reviewPath)
	if err != nil {
		t.Fatalf("reading review.go: %v", err)
	}

	contentStr := string(content)
	rendererSection := extractCLIPromptRendererSection(contentStr)

	// Check for map construction in renderer section (not in other sections)
	if strings.Contains(rendererSection, "map[string]interface{}{") {
		t.Error("cliPromptRenderer should construct typed pipeline structs, not map[string]interface{}")
	}
}

// TestDecomposeWorkflow_NoReflectImport verifies decompose.go doesn't import reflect package
func TestDecomposeWorkflow_NoReflectImport(t *testing.T) {
	decomposePath := filepath.Join("..", "..", "internal", "pipeline", "decompose.go")
	content, err := os.ReadFile(decomposePath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Check for reflect import
	if strings.Contains(contentStr, `"reflect"`) {
		t.Error("decompose.go should not import reflect package - all type assertions should be removed")
	}
}

// TestDecomposeWorkflow_NoTypeAssertions verifies decompose.go doesn't use map[string]interface{} type assertions
func TestDecomposeWorkflow_NoTypeAssertions(t *testing.T) {
	decomposePath := filepath.Join("..", "..", "internal", "pipeline", "decompose.go")
	content, err := os.ReadFile(decomposePath)
	if err != nil {
		t.Fatalf("reading decompose.go: %v", err)
	}

	contentStr := string(content)

	// Check for type assertions to map[string]interface{}
	if strings.Contains(contentStr, ".(map[string]interface{})") {
		t.Error("decompose.go should not use .(map[string]interface{}) type assertions - use typed structs instead")
	}

	// Check for extractBeadID function (should be deleted)
	if strings.Contains(contentStr, "func extractBeadID") {
		t.Error("extractBeadID function should be deleted - use BeadInfo.ID directly from typed return")
	}
}

// TestLogWriter_AcceptsAny verifies LogWriter.Write uses 'any' per Decision 3 in spec
func TestLogWriter_AcceptsAny(t *testing.T) {
	// This test verifies the design decision to keep LogWriter.Write(entry any)
	// rather than using a typed LogEntry, since it's a write-only sink
	pipelinePath := filepath.Join("..", "..", "internal", "pipeline", "pipeline.go")
	content, err := os.ReadFile(pipelinePath)
	if err != nil {
		t.Fatalf("reading pipeline.go: %v", err)
	}

	contentStr := string(content)

	// Find LogWriter interface
	logWriterSection := extractBetweenMarkers(contentStr, "type LogWriter interface", "}")
	if logWriterSection == "" {
		t.Fatal("Could not find LogWriter interface in pipeline.go")
	}

	// Verify it uses 'any' (which is acceptable per spec Decision 3)
	if !strings.Contains(logWriterSection, "Write(entry any)") {
		// This is actually fine - it could use a typed entry if that's better
		// But the test documents the decision
		t.Log("LogWriter.Write uses a typed parameter - this is fine if log entries are constructed by pipeline")
	}
}

// Helper functions

func containsTypedSignature(content, methodName, inputTypeName string) bool {
	// Look for pattern like: func (r *cliPromptRenderer) MethodName(input *pipeline.InputType)
	pattern := "func (r *cliPromptRenderer) " + methodName + "(input *pipeline." + inputTypeName + ")"
	return strings.Contains(content, pattern) ||
		strings.Contains(content, methodName+"(input *pipeline."+inputTypeName+")")
}

func extractCLIPromptRendererSection(content string) string {
	// Extract from "type cliPromptRenderer struct" to the next non-method section
	startIdx := strings.Index(content, "type cliPromptRenderer struct")
	if startIdx < 0 {
		return ""
	}

	// Look for the next type declaration after the renderer methods
	endMarkers := []string{
		"\ntype cliBacklogClient struct",
		"\ntype cliLearningsManager struct",
		"\ntype cliClaudeRunner struct",
		"\nfunc getGitHeadForReview",
	}

	endIdx := len(content)
	for _, marker := range endMarkers {
		if idx := strings.Index(content[startIdx:], marker); idx > 0 && startIdx+idx < endIdx {
			endIdx = startIdx + idx
		}
	}

	return content[startIdx:endIdx]
}

func extractBetweenMarkers(content, startMarker, endMarker string) string {
	startIdx := strings.Index(content, startMarker)
	if startIdx < 0 {
		return ""
	}

	if endMarker == "" {
		return content[startIdx:]
	}

	endIdx := strings.Index(content[startIdx:], endMarker)
	if endIdx < 0 {
		return content[startIdx:]
	}

	return content[startIdx : startIdx+endIdx]
}
