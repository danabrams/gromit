package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/specflow"
)

func TestBuildSpecStageContextUsesInjectedFactories(t *testing.T) {
	ctx := context.Background()
	specName := "sample"
	gromitDir := "custom-dir"

	storeFactoryCalled := false
	managerFactoryCalled := false

	storeFactory := func(dir string) (specflow.SpecStore, error) {
		storeFactoryCalled = true
		if dir != gromitDir {
			t.Fatalf("expected gromitDir %q, got %q", gromitDir, dir)
		}
		return &testSpecStore{}, nil
	}

	var createdManager *specflow.Manager
	managerFactory := func(store specflow.SpecStore) *specflow.Manager {
		managerFactoryCalled = true
		if store == nil {
			t.Fatal("expected non-nil store")
		}
		createdManager = specflow.NewManager(store)
		return createdManager
	}

	stageCtx, err := BuildSpecStageContext(ctx, &config.Config{}, specName, gromitDir, storeFactory, managerFactory)
	if err != nil {
		t.Fatalf("BuildSpecStageContext returned error: %v", err)
	}
	if !storeFactoryCalled || !managerFactoryCalled {
		t.Fatal("expected factories to be invoked")
	}
	if stageCtx == nil {
		t.Fatal("expected stage context")
	}
	if stageCtx.SpecName != specName {
		t.Fatalf("expected spec name %q, got %q", specName, stageCtx.SpecName)
	}
	if stageCtx.Stage != specflow.StagePlanning {
		t.Fatalf("expected planning stage, got %s", stageCtx.Stage)
	}
	if !stageCtx.FreshStart {
		t.Fatalf("expected fresh start")
	}
	if stageCtx.Manager != createdManager {
		t.Fatalf("expected manager returned by factory")
	}
}

type testSpecStore struct{}

func (testSpecStore) Stage(context.Context, string) (specflow.Stage, error) {
	return "", specflow.ErrStageNotFound
}

func (testSpecStore) StoreStage(context.Context, string, specflow.Stage) error {
	return nil
}

type fakeSpecBranchCreator struct {
	calledWith string
}

func (f *fakeSpecBranchCreator) CreateOrCheckoutSpecBranch(_ context.Context, specBranchName string) error {
	f.calledWith = specBranchName
	return nil
}

func TestEnsureSpecBranchUsesInjectedCreatorFactory(t *testing.T) {
	ctx := context.Background()
	repoDir := "repo"
	cfg := &config.Config{Git: config.GitConfig{BaseBranch: "develop"}}
	stageCtx := &StageContext{SpecName: "feature-x"}

	origFallback := SpecBranchCreatorFactory
	SpecBranchCreatorFactory = func(string, *config.Config) (SpecBranchCreator, error) {
		return nil, fmt.Errorf("fallback should not run")
	}
	defer func() { SpecBranchCreatorFactory = origFallback }()

	var observedRepo string
	var observedCfg *config.Config
	creator := &fakeSpecBranchCreator{}
	factory := func(dir string, cfgParam *config.Config) (SpecBranchCreator, error) {
		observedRepo = dir
		observedCfg = cfgParam
		return creator, nil
	}

	if err := EnsureSpecBranch(ctx, cfg, stageCtx, repoDir, factory); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if observedRepo != repoDir {
		t.Fatalf("expected repoDir %q, got %q", repoDir, observedRepo)
	}
	if observedCfg != cfg {
		t.Fatal("expected factory to receive the original config")
	}
	expectedBranch := "gromit/spec-feature-x"
	if creator.calledWith != expectedBranch {
		t.Fatalf("expected branch %q, got %q", expectedBranch, creator.calledWith)
	}
}
