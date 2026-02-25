package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/danabrams/gromit/internal/bead"
	"github.com/danabrams/gromit/internal/config"
	"github.com/danabrams/gromit/internal/frontmatter"
	"github.com/danabrams/gromit/internal/prompt"
	"github.com/danabrams/gromit/internal/provider"
	"github.com/danabrams/gromit/internal/scope"
	"github.com/danabrams/gromit/internal/specgate"
	"github.com/spf13/cobra"
)

var verifySpecCreateBeads bool
var verifySpecGateRunner = runSpecGate
var verifySpecFixBeadsFn = createSpecGateFixBeads
var verifySpecBuildRouterFromConfig = provider.BuildRouterFromConfig

var verifySpecCmd = &cobra.Command{
	Use:   "verify-spec <spec>",
	Short: "Verify a spec's acceptance criteria",
	Args:  cobra.ExactArgs(1),
	RunE:  runVerifySpec,
}

func init() {
	verifySpecCmd.Flags().BoolVar(&verifySpecCreateBeads, "create-beads", false, "Create fix beads for failing criteria")
	rootCmd.AddCommand(verifySpecCmd)
}

func runVerifySpec(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	specName := args[0]
	specsDir := resolveSpecsDir(cfg)

	if err := scope.ValidateSpec(specsDir, specName); err != nil {
		return fmt.Errorf("validating spec: %w", err)
	}

	labels := scope.ResolveSpec(specName)
	if len(labels) == 0 {
		return fmt.Errorf("no label found for spec %q", specName)
	}

	criteria, criteriaBlock, specBody, err := loadSpecGateInputs(specsDir, specName)
	if err != nil {
		return err
	}

	ctx := commandContext(cmd)

	verdict, err := verifySpecGateRunner(ctx, cfg, specName, criteria, criteriaBlock, specBody)
	if err != nil {
		return fmt.Errorf("running spec gate: %w", err)
	}

	printSpecGateVerdict(verdict)

	if verdict.Passed {
		return nil
	}

	if verifySpecCreateBeads {
		createdIDs, err := verifySpecFixBeadsFn(ctx, specName, verdict)
		if err != nil {
			return fmt.Errorf("creating fix beads: %w", err)
		}
		if len(createdIDs) > 0 {
			fmt.Printf("Created fix beads: %s\n", strings.Join(createdIDs, ", "))
		}
	}

	return fmt.Errorf("spec gate failed")
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd == nil {
		return context.Background()
	}
	ctx := cmd.Context()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

var acceptanceCriteriaNumberedRE = regexp.MustCompile(`^\d+[.)]\s+(.+)$`)

func extractAcceptanceCriteria(body string) ([]string, string) {
	lines := strings.Split(body, "\n")
	inSection := false
	var blockLines []string
	criteria := make([]string, 0)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if inSection {
				break
			}
			if strings.EqualFold(trimmed, "## Acceptance Criteria") {
				inSection = true
			}
			continue
		}

		if !inSection {
			continue
		}

		blockLines = append(blockLines, line)

		switch {
		case strings.HasPrefix(trimmed, "- "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
		case strings.HasPrefix(trimmed, "* "):
			criteria = append(criteria, strings.TrimSpace(strings.TrimPrefix(trimmed, "* ")))
		default:
			if matches := acceptanceCriteriaNumberedRE.FindStringSubmatch(trimmed); len(matches) == 2 {
				criteria = append(criteria, strings.TrimSpace(matches[1]))
			}
		}
	}

	block := strings.TrimSpace(strings.Join(blockLines, "\n"))

	return criteria, block
}

func loadSpecGateInputs(specsDir, specName string) ([]string, string, string, error) {
	specPath := filepath.Join(specsDir, specName+".md")
	_, body, err := frontmatter.ReadFile(specPath)
	if err != nil {
		return nil, "", "", fmt.Errorf("reading spec: %w", err)
	}

	criteria, block := extractAcceptanceCriteria(body)
	if len(criteria) == 0 {
		return nil, block, body, fmt.Errorf("spec %q has no acceptance criteria", specName)
	}

	return criteria, block, body, nil
}

func printSpecGateVerdict(verdict *specgate.GateVerdict) {
	if verdict == nil {
		return
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "CRITERION\tSTATUS\tEVIDENCE")
	for _, result := range verdict.Results {
		status := specGateStatusPass
		if !result.Passed {
			status = specGateStatusFail
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\n", result.Criterion, status, result.Evidence)
	}
	_ = writer.Flush()
}

func runSpecGate(ctx context.Context, cfg *config.Config, specName string, criteria []string, criteriaBlock string, specBody string) (*specgate.GateVerdict, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	renderer, err := newSpecGateRenderer(cfg)
	if err != nil {
		return nil, err
	}

	router, err := buildVerifySpecRouter(cfg)
	if err != nil {
		return nil, err
	}

	workDir, _ := os.Getwd()
	model := cfg.Models.P1

	gate := &specgate.Gate{
		Model:     model,
		MaxCycles: 1,
		RunTests: func(ctx context.Context) (string, error) {
			return runSpecGateTests(ctx, workDir)
		},
		GetDiff: func(ctx context.Context) (string, error) {
			return runSpecGateDiff(ctx, workDir)
		},
		RenderPrompt: func(ctx context.Context, specName, testOutput, diff string, criteria []string) (string, error) {
			acceptance := criteriaBlock
			if strings.TrimSpace(acceptance) == "" {
				acceptance = formatAcceptanceCriteria(criteria)
			}
			promptCtx := &prompt.SpecGateContext{
				SpecCriteria:       specBody,
				TestOutput:         testOutput,
				CumulativeDiff:     diff,
				AcceptanceCriteria: acceptance,
			}
			return renderer.RenderSpecGate(promptCtx)
		},
		InvokeLLM: func(ctx context.Context, model, promptText string) ([]byte, error) {
			return invokeSpecGateLLM(ctx, router, model, promptText)
		},
	}

	return gate.Run(ctx, specName, criteria)
}

func newSpecGateRenderer(cfg *config.Config) (*prompt.Renderer, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	templatesDir := resolveTemplatesDir(cfg)
	specsDir := resolveSpecsDir(cfg)
	claudeMDPath := resolveProjectClaudeMD(cfg)
	gromitDir := resolveGromitDir(cfg)

	renderer, err := prompt.NewRenderer(templatesDir, specsDir, claudeMDPath, gromitDir)
	if err != nil {
		return nil, fmt.Errorf("creating prompt renderer: %w", err)
	}
	return renderer, nil
}

var verifySpecCmdRunner = defaultVerifySpecCmdRunner

const (
	specGateTestCommand = "go test -tags acceptance ./..."
	specGateDiffCommand = "git diff"
	specGateStatusPass  = "PASS"
	specGateStatusFail  = "FAIL"
)

func runSpecGateTests(ctx context.Context, workDir string) (string, error) {
	return runSpecGateCommand(ctx, workDir, specGateTestCommand)
}

func runSpecGateDiff(ctx context.Context, workDir string) (string, error) {
	return runSpecGateCommand(ctx, workDir, specGateDiffCommand)
}

func runSpecGateCommand(ctx context.Context, workDir string, command string) (string, error) {
	stdout, stderr, exitCode, err := verifySpecCmdRunner(ctx, command, workDir)
	if err != nil {
		return "", err
	}

	output := strings.TrimSpace(strings.Join([]string{stdout, stderr}, "\n"))
	if exitCode != 0 {
		if output == "" {
			output = fmt.Sprintf("%s (exit %d)", command, exitCode)
		} else {
			output = fmt.Sprintf("%s (exit %d)\n%s", command, exitCode, output)
		}
	}

	return strings.TrimSpace(output), nil
}

func formatAcceptanceCriteria(criteria []string) string {
	if len(criteria) == 0 {
		return ""
	}

	lines := make([]string, 0, len(criteria))
	for _, item := range criteria {
		lines = append(lines, "- "+item)
	}
	return strings.Join(lines, "\n")
}

func invokeSpecGateLLM(ctx context.Context, router *provider.Router, model string, promptText string) ([]byte, error) {
	if router == nil {
		return nil, fmt.Errorf("router is nil")
	}

	tier := provider.TierFromLegacyModel(model)
	p, _ := router.Select("build", tier)
	if p == nil {
		return nil, fmt.Errorf("no providers available for tier %q", tier)
	}

	result, err := p.Run(ctx, promptText, tier)
	if err != nil && p.IsUsageLimitError(result, err) {
		router.MarkUnavailable(p.Name())
		p, _ = router.Select("build", tier)
		if p != nil {
			result, err = p.Run(ctx, promptText, tier)
		}
	}
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("provider returned nil result")
	}

	return []byte(result.Output), nil
}

func buildVerifySpecRouter(cfg *config.Config) (*provider.Router, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if reflect.ValueOf(verifySpecBuildRouterFromConfig).Pointer() != reflect.ValueOf(provider.BuildRouterFromConfig).Pointer() {
		return verifySpecBuildRouterFromConfig(cfg)
	}
	return provider.BuildRouterFromConfig(cfg)
}

func createSpecGateFixBeads(ctx context.Context, specName string, verdict *specgate.GateVerdict) ([]string, error) {
	if verdict == nil {
		return []string{}, nil
	}
	failures := verdict.FailedCriteria()
	if len(failures) == 0 {
		return []string{}, nil
	}

	creator, err := newSpecGateBeadCreator()
	if err != nil {
		return nil, err
	}

	return specgate.SynthesizeFixBeads(ctx, specName, failures, "P1", creator)
}

type specGateBeadCreator struct {
	client *bead.Client
}

func newSpecGateBeadCreator() (*specGateBeadCreator, error) {
	client, err := bead.NewClient()
	if err != nil {
		return nil, err
	}
	return &specGateBeadCreator{client: client}, nil
}

func (c *specGateBeadCreator) Create(ctx context.Context, title, description, priority string, labels []string) (string, error) {
	if c == nil || c.client == nil {
		return "", fmt.Errorf("bead client is nil")
	}

	priorityInt, err := parseBeadPriority(priority)
	if err != nil {
		return "", err
	}

	expectedOutputs := bead.ExpectedOutputsOrTitle(nil, title)
	bead, err := c.client.CreateWithParentAndDescription(title, priorityInt, labels, expectedOutputs, "", description)
	if err != nil {
		return "", err
	}
	if bead == nil {
		return "", fmt.Errorf("bead creation returned nil")
	}
	return bead.ID, nil
}

func parseBeadPriority(priority string) (int, error) {
	trimmed := strings.TrimSpace(priority)
	if trimmed == "" {
		return 0, fmt.Errorf("priority is empty")
	}
	if strings.HasPrefix(strings.ToUpper(trimmed), "P") {
		trimmed = strings.TrimSpace(trimmed[1:])
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid priority %q", priority)
	}
	return value, nil
}

func defaultVerifySpecCmdRunner(ctx context.Context, command string, workDir string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader("")
	cmd.Env = append(
		os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"CI=1",
		"NONINTERACTIVE=1",
		"TERM=dumb",
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.String(), stderr.String(), exitErr.ExitCode(), nil
		}
		return stdout.String(), stderr.String(), -1, err
	}
	return stdout.String(), stderr.String(), 0, nil
}

var _ specgate.BeadCreator = (*specGateBeadCreator)(nil)
