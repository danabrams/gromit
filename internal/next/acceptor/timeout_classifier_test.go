package acceptor

import (
	"testing"
	"time"
)

func TestClassifyKeywordComplexity_Outcomes(t *testing.T) {
	for _, keyword := range complexityKeywords {
		name := keyword
		t.Run("keyword "+name, func(t *testing.T) {
			input := "The system must " + name + " the flow"
			if got := ClassifyCriterionComplexity(input); got != complexityComplex {
				t.Fatalf("keyword %q classified as %q", name, got)
			}
		})
	}

	simpleExamples := []string{
		"check that type X is present",
		"validate that the key exists",
		"ensure the field maps correctly",
	}

	for idx, example := range simpleExamples {
		t.Run("simple example #"+string(rune(idx+'1')), func(t *testing.T) {
			if got := ClassifyCriterionComplexity(example); got != complexitySimple {
				t.Fatalf("simple example %q classified as %q", example, got)
			}
		})
	}
}

func TestComputeTimeout_FormulaRanges(t *testing.T) {
	cfg := DefaultTimeoutConfig()
	baseDuration := time.Duration(cfg.BaseSeconds) * time.Second
	hardMax := time.Duration(cfg.HardMaximumSecs) * time.Second

	diffBytes := 500_000
	simpleCriterion := "ensure the type exists"
	simpleTimeout := ComputeCriterionTimeout(cfg, diffBytes, simpleCriterion)

	if simpleTimeout <= baseDuration {
		t.Fatalf("simple timeout %v not above base %v", simpleTimeout, baseDuration)
	}
	if simpleTimeout >= hardMax {
		t.Fatalf("simple timeout %v exceeded hard maximum %v", simpleTimeout, hardMax)
	}

	complexCriterion := "end-to-end pipeline behavior"
	complexTimeout := ComputeCriterionTimeout(cfg, diffBytes, complexCriterion)
	bonusDuration := time.Duration(cfg.ComplexityBonusSecs) * time.Second
	if diff := complexTimeout - simpleTimeout; diff != bonusDuration {
		t.Fatalf("complex bonus diff %v, want %v", diff, bonusDuration)
	}
	if complexTimeout >= hardMax {
		t.Fatalf("complex timeout %v should stay below hard maximum %v", complexTimeout, hardMax)
	}

	zeroDiffTimeout := ComputeCriterionTimeout(cfg, 0, simpleCriterion)
	if zeroDiffTimeout != baseDuration {
		t.Fatalf("zero diff timeout %v, want base %v", zeroDiffTimeout, baseDuration)
	}

	largeDiff := cfg.RateConstant * 1_000
	largeTimeout := ComputeCriterionTimeout(cfg, largeDiff, "integration across workflow")
	if largeTimeout != hardMax {
		t.Fatalf("large diff timeout %v, want hard max %v", largeTimeout, hardMax)
	}
}
