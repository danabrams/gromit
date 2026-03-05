package runner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/specbranch"
	"github.com/danabrams/gromit/internal/specflow"
)

type SpecflowStoreFactoryFn func(gromitDir string) (specflow.SpecStore, error)
type SpecflowManagerFactoryFn func(store specflow.SpecStore) *specflow.Manager
type SpecBranchCreatorFactoryFn func(repoDir string, cfg *config.Config) (SpecBranchCreator, error)

var (
	SpecflowStoreFactory     SpecflowStoreFactoryFn     = specflow.NewFileStore
	SpecflowManagerFactory   SpecflowManagerFactoryFn   = specflow.NewManager
	SpecBranchCreatorFactory SpecBranchCreatorFactoryFn = defaultSpecBranchCreator
)

type SpecBranchCreator interface {
	CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error
}

func BuildSpecStageContext(ctx context.Context, cfg *config.Config, specName, gromitDir string, storeFactory SpecflowStoreFactoryFn, managerFactory SpecflowManagerFactoryFn) (*StageContext, error) {
	if cfg == nil {
		return nil, nil
	}
	if storeFactory == nil {
		storeFactory = SpecflowStoreFactory
	}
	if managerFactory == nil {
		managerFactory = SpecflowManagerFactory
	}
	store, err := storeFactory(gromitDir)
	if err != nil {
		return nil, err
	}
	manager := managerFactory(store)
	stage, bootstrapped, err := manager.ResumeWithBootstrap(ctx, specName)
	if err != nil {
		return nil, err
	}
	return &StageContext{
		SpecName:   specName,
		Stage:      stage,
		FreshStart: bootstrapped,
		Manager:    manager,
	}, nil
}

func EnsureSpecBranch(ctx context.Context, cfg *config.Config, stageCtx *StageContext, repoDir string, creatorFactory SpecBranchCreatorFactoryFn) error {
	if stageCtx == nil || stageCtx.SpecName == "" {
		return nil
	}
	branchName, err := specBranchNameForSpec(cfg, stageCtx.SpecName)
	if err != nil {
		return err
	}
	if creatorFactory == nil {
		creatorFactory = SpecBranchCreatorFactory
	}
	creator, err := creatorFactory(repoDir, cfg)
	if err != nil {
		return err
	}
	return creator.CreateOrCheckoutSpecBranch(ctx, branchName)
}

func defaultSpecBranchCreator(repoDir string, cfg *config.Config) (SpecBranchCreator, error) {
	baseBranch := config.DefaultBaseBranch
	if cfg != nil && cfg.Git.BaseBranch != "" {
		baseBranch = cfg.Git.BaseBranch
	}
	return specbranch.NewGitOps(repoDir, baseBranch), nil
}

func specBranchNameForSpec(cfg *config.Config, specName string) (string, error) {
	baseBranch := config.DefaultBaseBranch
	if cfg != nil && cfg.Git.BaseBranch != "" {
		baseBranch = cfg.Git.BaseBranch
	}
	router := specbranch.NewRouter(baseBranch)
	label := fmt.Sprintf("spec:%s", specName)
	return router.BranchForLabels([]string{label})
}
