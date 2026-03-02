package config

import (
	"bytes"
	"fmt"
	"os/exec"
)

var supplementaryConfigFiles = []string{
	"compatibility_resolution.go",
	"gemini_config.go",
	"profile_defaults.go",
	"config_normalize.go",
	"config_defaults.go",
	"config_accessors.go",
}

func EnsureSupplementaryConfigFilesFormatted() error {
	if err := runGofmtCheck(supplementaryConfigFiles); err != nil {
		return fmt.Errorf("supplementary config gofmt check failed: %w", err)
	}
	return nil
}

func runGofmtCheck(files []string) error {
	args := append([]string{"-l"}, files...)
	cmd := exec.Command("gofmt", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("running gofmt -l: %w", err)
	}
	if len(bytes.TrimSpace(output)) > 0 {
		return fmt.Errorf("gofmt -l reported unformatted files:\n%s", output)
	}
	return nil
}
