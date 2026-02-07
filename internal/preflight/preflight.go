package preflight

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/config"
)

// Checker handles tool availability checks and installation
type Checker struct {
	cfg config.PreflightConfig
	out io.Writer
}

// NewChecker creates a new preflight checker
func NewChecker(cfg config.PreflightConfig, output io.Writer) (*Checker, error) {
	if output == nil {
		output = os.Stdout
	}
	return &Checker{
		cfg: cfg,
		out: output,
	}, nil
}

// Check verifies required tools are available and attempts installation if missing
func (c *Checker) Check(commands []string) error {
	if c == nil {
		return fmt.Errorf("preflight checker is nil")
	}
	if len(commands) == 0 {
		return nil // Nothing to validate
	}

	tools := c.extractTools(commands)
	if len(tools) == 0 {
		return nil
	}

	missingTools := c.checkAvailability(tools)
	if len(missingTools) == 0 {
		return nil // All tools available
	}

	// Display pre-flight status
	c.printStatus(tools, missingTools)

	// Handle missing tools based on config
	return c.handleMissing(missingTools)
}

// extractTools parses tool names from validation commands
func (c *Checker) extractTools(commands []string) map[string]bool {
	tools := make(map[string]bool)

	// Explicit tools from config take precedence
	if len(c.cfg.Tools) > 0 {
		for _, tool := range c.cfg.Tools {
			tools[tool] = true
		}
		return tools
	}

	// Extract tools from commands
	toolNames := []string{
		"go", "node", "npm", "pnpm", "yarn", "python", "python3",
		"rust", "cargo", "ruby", "java", "javac", "gradle", "maven",
		"docker", "git", "mise", "curl", "wget",
	}

	for _, cmd := range commands {
		for _, tool := range toolNames {
			// Match tool name as word boundary
			pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(tool))
			if matched, _ := regexp.MatchString(pattern, cmd); matched {
				tools[tool] = true
			}
		}
	}

	return tools
}

// checkAvailability returns list of missing tools
func (c *Checker) checkAvailability(tools map[string]bool) []string {
	missing := []string{}

	for tool := range tools {
		if !c.toolExists(tool) {
			missing = append(missing, tool)
		}
	}

	return missing
}

// toolExists checks if a tool is available in PATH
func (c *Checker) toolExists(tool string) bool {
	cmd := exec.Command("which", tool)
	return cmd.Run() == nil
}

// printStatus displays the pre-flight check results
func (c *Checker) printStatus(allTools map[string]bool, missing []string) {
	fmt.Fprintln(c.out, "\nPre-flight check:")

	missingSet := make(map[string]bool)
	for _, t := range missing {
		missingSet[t] = true
	}

	for tool := range allTools {
		if missingSet[tool] {
			fmt.Fprintf(c.out, "  ✗ %s not found\n", tool)
		} else {
			fmt.Fprintf(c.out, "  ✓ %s\n", tool)
		}
	}
}

// handleMissing prompts user to install missing tools
func (c *Checker) handleMissing(missing []string) error {
	mode := c.cfg.AutoInstall
	if mode == "" {
		mode = "ask" // Default
	}

	switch mode {
	case "always":
		return c.autoInstall(missing)
	case "never":
		return fmt.Errorf("missing tools: %s. Install manually and try again",
			strings.Join(missing, ", "))
	case "ask":
		return c.promptInstall(missing)
	default:
		return fmt.Errorf("invalid preflight.auto_install: %s (expected ask, always, or never)", mode)
	}
}

// autoInstall attempts to install missing tools using available package managers
func (c *Checker) autoInstall(missing []string) error {
	for _, tool := range missing {
		fmt.Fprintf(c.out, "\nInstalling %s...\n", tool)

		// Try common installation methods
		if err := c.tryInstall(tool); err != nil {
			return fmt.Errorf("failed to install %s: %w", tool, err)
		}
	}
	return nil
}

// tryInstall attempts installation with common package managers
func (c *Checker) tryInstall(tool string) error {
	// Special handling for common tools
	switch tool {
	case "mise":
		return c.runCmd("mise", "install")
	case "go":
		if c.fileExists("go.mod") {
			return c.runCmd("mise", "install")
		}
	case "npm", "node":
		if c.fileExists("package.json") {
			return c.runCmd("npm", "install")
		}
	case "pnpm":
		if c.fileExists("pnpm-lock.yaml") {
			return c.runCmd("pnpm", "install")
		}
	}

	// Try generic package managers
	// Try to detect and use system package manager
	if c.toolExists("apt-get") {
		return c.runCmd("sudo", "apt-get", "install", "-y", tool)
	}
	if c.toolExists("brew") {
		return c.runCmd("brew", "install", tool)
	}

	return fmt.Errorf("no installation method found for %s", tool)
}

// promptInstall asks the user what to do with missing tools
func (c *Checker) promptInstall(missing []string) error {
	fmt.Fprintf(c.out, "\nMissing tools: %s\n\n", strings.Join(missing, ", "))
	fmt.Fprintln(c.out, "Options:")

	installAvailable := c.hasInstallMethod(missing)
	if installAvailable {
		fmt.Fprintln(c.out, "  [1] Try to install automatically")
	}
	fmt.Fprintln(c.out, "  [2] Skip validation and continue")
	fmt.Fprintln(c.out, "  [3] Abort")

	if !installAvailable {
		fmt.Fprintln(c.out, "  Note: No automatic installation method detected")
	}

	choice := c.prompt("\nChoice [2]: ")
	if choice == "" {
		choice = "2"
	}

	switch choice {
	case "1":
		if !installAvailable {
			return fmt.Errorf("no installation method available for: %s", strings.Join(missing, ", "))
		}
		return c.autoInstall(missing)
	case "2":
		return nil // Skip validation
	case "3":
		return fmt.Errorf("aborted due to missing tools: %s", strings.Join(missing, ", "))
	default:
		return fmt.Errorf("invalid choice: %s", choice)
	}
}

// hasInstallMethod checks if any tool has an available installation method
func (c *Checker) hasInstallMethod(missing []string) bool {
	if c.fileExists("mise.toml") || c.fileExists(".tool-versions") {
		return true
	}
	if c.fileExists("package.json") || c.fileExists("pnpm-lock.yaml") {
		return true
	}
	if c.fileExists("go.mod") {
		return true
	}
	if c.toolExists("apt-get") || c.toolExists("brew") {
		return true
	}
	return false
}

// prompt displays a prompt and reads user input
func (c *Checker) prompt(msg string) string {
	fmt.Fprint(c.out, msg)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

// runCmd executes a command and returns any error
func (c *Checker) runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = c.out
	cmd.Stderr = c.out
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// fileExists checks if a file exists
func (c *Checker) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
