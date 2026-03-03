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
    provider := &fakeCriteriaProvider{}
    updater := &fakeBeadUpdater{}

    enricher := NewLLMCriteriaEnricher(provider, loader, updater)
    bead := &bead.Bead{ID: "bead-1", Labels: []string{"spec:alpha"}, Title: "Title"}

    if _, err := enricher.Enrich(context.Background(), bead); err != nil {
        t.Fatalf("Enrich returned error: %v", err)
    }

    if !strings.Contains(provider.lastPrompt, "Spec body") {
        t.Fatalf("prompt %q missing spec context", provider.lastPrompt)
    }
    if provider.lastTier != provider.TierLow {
        t.Fatalf("expected TierLow, got %q", provider.lastTier)
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
    lastCriteria []string
}

func (u *fakeBeadUpdater) UpdateAcceptanceCriteria(ctx context.Context, b *bead.Bead, criteria []string) (*bead.Bead, error) {
    u.lastCriteria = append([]string(nil), criteria...)
    clone := *b
    clone.ExpectedOutputs = append([]string(nil), criteria...)
    clone.AcceptanceCriteria = strings.Join(criteria, "\n")
    return &clone, nil
}
