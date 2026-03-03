package prepare

import (
	"context"
	"strings"
	"testing"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/provider"
)

func TestLLMCriteriaEnricher_UsesSpecContext(t *testing.T) {
	specDoc := &Document{Title: "Spec Title", Body: "# Spec\nSpec body"}
	planDoc := &Document{Title: "Plan Title", Body: "# Plan\nPlan body"}
	loader := &fakeSpecLoader{
		specDoc:   specDoc,
		planDoc:   planDoc,
		specFound: true,
		planFound: true,
	}
	fakeProvider := &fakeCriteriaProvider{}
	updater := &fakeBeadUpdater{}

	enricher := NewLLMCriteriaEnricher(fakeProvider, loader, updater)
	bead := &bead.Bead{ID: "bead-1", Labels: []string{"spec:alpha"}, Title: "Title"}

	if err := enricher.Enrich(context.Background(), bead); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	if !strings.Contains(fakeProvider.lastPrompt, "Spec body") {
		t.Fatalf("prompt %q missing spec context", fakeProvider.lastPrompt)
	}
	if fakeProvider.lastTier != provider.TierLow {
		t.Fatalf("expected TierLow, got %q", fakeProvider.lastTier)
	}
}

func TestLLMCriteriaEnricher_FallbacksToTitleAndDescription(t *testing.T) {
	loader := &fakeSpecLoader{}
	fakeProvider := &fakeCriteriaProvider{}
	updater := &fakeBeadUpdater{}

	enricher := NewLLMCriteriaEnricher(fakeProvider, loader, updater)
	bead := &bead.Bead{
		ID:          "fallback-bead",
		Title:       "Fallback Title",
		Description: "Fallback Description",
	}

	if err := enricher.Enrich(context.Background(), bead); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}

	expected := "Title: Fallback Title\nDescription: Fallback Description"
	if !strings.Contains(fakeProvider.lastPrompt, expected) {
		t.Fatalf("fallback prompt %q missing block %q", fakeProvider.lastPrompt, expected)
	}
}

func TestLLMCriteriaEnricher_PersistsExpectedOutputs(t *testing.T) {
	loader := &fakeSpecLoader{}
	fakeProvider := &fakeCriteriaProvider{}
	updater := &fakeBeadUpdater{}
	enricher := NewLLMCriteriaEnricher(fakeProvider, loader, updater)
	bead := &bead.Bead{ID: "persist-bead", Title: "Persist criteria"}

	if err := enricher.Enrich(context.Background(), bead); err != nil {
		t.Fatalf("Enrich returned error: %v", err)
	}
	if updater.lastUpdateID != bead.ID {
		t.Fatalf("updated bead ID = %q, want %q", updater.lastUpdateID, bead.ID)
	}
	if got, want := len(updater.lastUpdatedOutputs), 2; got != want {
		t.Fatalf("persisted outputs = %v, want %d entries", updater.lastUpdatedOutputs, want)
	}
	if got := len(bead.ExpectedOutputs); got != len(updater.lastUpdatedOutputs) {
		t.Fatalf("enriched bead outputs = %d, want %d", got, len(updater.lastUpdatedOutputs))
	}
}

type fakeCriteriaProvider struct {
	lastPrompt string
	lastTier   string
}

func (f *fakeCriteriaProvider) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	f.lastPrompt = prompt
	f.lastTier = tier
	return &provider.Result{Success: true, Output: "criterion 1\ncriterion 2\n"}, nil
}

type fakeSpecLoader struct {
	specFound bool
	planFound bool
	specDoc   *Document
	planDoc   *Document
}

func (f *fakeSpecLoader) LoadSpec(ctx context.Context, specID string) (*Document, bool, error) {
	return f.specDoc, f.specFound, nil
}

func (f *fakeSpecLoader) LoadPlan(ctx context.Context, specID string) (*Document, bool, error) {
	return f.planDoc, f.planFound, nil
}

type fakeBeadUpdater struct {
	lastUpdateID       string
	lastUpdatedOutputs []string
}

func (u *fakeBeadUpdater) UpdateExpectedOutputs(ctx context.Context, id string, outputs []string) error {
	u.lastUpdateID = id
	u.lastUpdatedOutputs = append([]string(nil), outputs...)
	return nil
}
