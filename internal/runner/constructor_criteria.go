package runner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/pipeline/prepare"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/tracker"
)

const criteriaPhaseName = "gate"

func newGateCriteriaEnricher(cfg *config.Config, router *provider.Router, trackerClient tracker.Client) *prepare.LLMCriteriaEnricher {
	if cfg == nil || router == nil || trackerClient == nil {
		return nil
	}

	beadClient := bead.UnwrapBDAdapter(trackerClient)
	if beadClient == nil {
		return nil
	}

	providerAdapter := newCriteriaLLMProvider(router)
	if providerAdapter == nil {
		return nil
	}

	loader := newDiskSpecLoader(cfg.Paths.Specs, cfg.Paths.Plans)
	return prepare.NewLLMCriteriaEnricher(providerAdapter, loader, beadClient)
}

type criteriaLLMProvider struct {
	router *provider.Router
}

func newCriteriaLLMProvider(router *provider.Router) *criteriaLLMProvider {
	if router == nil {
		return nil
	}
	return &criteriaLLMProvider{router: router}
}

func (p *criteriaLLMProvider) Run(ctx context.Context, prompt, tier string) (*provider.Result, error) {
	if p == nil || p.router == nil {
		return nil, fmt.Errorf("criteria provider router is not configured")
	}

	selected, model := p.router.Select(criteriaPhaseName, tier)
	if selected == nil {
		return nil, fmt.Errorf("no provider available for criteria enrichment")
	}
	if model == "" {
		model = tier
	}
	return selected.Run(ctx, prompt, model)
}

type diskSpecLoader struct {
	specsDir string
	plansDir string
}

func newDiskSpecLoader(specsDir, plansDir string) *diskSpecLoader {
	if strings.TrimSpace(specsDir) == "" && strings.TrimSpace(plansDir) == "" {
		return nil
	}
	return &diskSpecLoader{
		specsDir: specsDir,
		plansDir: plansDir,
	}
}

func (d *diskSpecLoader) LoadSpec(_ context.Context, specID string) (*prepare.Document, bool, error) {
	if d == nil || strings.TrimSpace(specID) == "" || strings.TrimSpace(d.specsDir) == "" {
		return nil, false, nil
	}
	return loadMarkdownDocument(filepath.Join(d.specsDir, specID+".md"))
}

func (d *diskSpecLoader) LoadPlan(_ context.Context, specID string) (*prepare.Document, bool, error) {
	if d == nil || strings.TrimSpace(specID) == "" || strings.TrimSpace(d.plansDir) == "" {
		return nil, false, nil
	}
	return loadMarkdownDocument(filepath.Join(d.plansDir, specID+".md"))
}

func loadMarkdownDocument(path string) (*prepare.Document, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return nil, false, nil
	}
	title, body, err := parseMarkdownDocument(text)
	if err != nil {
		return nil, false, err
	}
	return &prepare.Document{Title: title, Body: body}, true, nil
}

func parseMarkdownDocument(text string) (string, string, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	inFrontMatter := false
	firstLine := true
	var bodyLines []string
	var title string

	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && line == "---" {
			inFrontMatter = true
			firstLine = false
			continue
		}
		firstLine = false
		if inFrontMatter {
			if line == "---" {
				inFrontMatter = false
			}
			continue
		}
		if title == "" && strings.HasPrefix(line, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		bodyLines = append(bodyLines, line)
	}

	if err := scanner.Err(); err != nil {
		return "", "", err
	}

	return title, strings.TrimSpace(strings.Join(bodyLines, "\n")), nil
}
