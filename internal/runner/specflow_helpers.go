package runner

import (
	"context"
	"fmt"

	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/runner/specbranch"
	"github.com/danabrams/gromit/internal/specflow"
)

var (
	SpecflowStoreFactory     = specflow.NewFileStore
	SpecflowManagerFactory   = specflow.NewManager
	SpecBranchCreatorFactory = defaultSpecBranchCreator
)

type SpecBranchCreator interface {
	CreateOrCheckoutSpecBranch(ctx context.Context, specBranchName string) error
}

func BuildSpecStageContext(ctx context.Context, cfg *config.Config, specName, gromitDir string, storeFactory func(string) (specflow.SpecStore, error)) (*StageContext, error) {
	if cfg == nil {
		return nil, nil
	}
	if storeFactory == nil {
		storeFactory = SpecflowStoreFactory
	}
	store, err := storeFactory(gromitDir)
	if err != nil {
		return nil, err
	}
	manager := SpecflowManagerFactory(store)
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

func EnsureSpecBranch(ctx context.Context, cfg *config.Config, stageCtx *StageContext, repoDir string, branchFactory func(string, *config.Config) (SpecBranchCreator, error)) error {
	if stageCtx == nil || stageCtx.SpecName == "" {
		return nil
	}
	branchName, err := specBranchNameForSpec(cfg, stageCtx.SpecName)
	if err != nil {
		return err
	}
	if branchFactory == nil {
		branchFactory = SpecBranchCreatorFactory
	}
	creator, err := branchFactory(repoDir, cfg)
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
