package validate

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/runner/runtypes"
)

// NewAutoFixFn returns an AutoFixFn that formats Go files changed since startCommit.
func NewAutoFixFn(runner func(ctx context.Context, name string, args ...string) (string, error)) runtypes.AutoFixFn {
	if runner == nil {
		return nil
	}

	return func(startCommit string) error {
		startCommit = strings.TrimSpace(startCommit)
		if startCommit == "" {
			return nil
		}

		diffOutput, err := runner(context.Background(), "git", "diff", "--name-only", startCommit)
		if err != nil {
			return err
		}

		files := collectGoFiles(diffOutput)
		for _, file := range files {
			_, _ = runner(context.Background(), "gofmt", "-w", file)
			_, _ = runner(context.Background(), "goimports", "-w", file)
		}
		return nil
	}
}

func collectGoFiles(diffOutput string) []string {
	seen := map[string]struct{}{}
	var files []string
	for _, line := range strings.Split(diffOutput, "\n") {
		file := strings.TrimSpace(line)
		if file == "" {
			continue
		}
		if filepath.Ext(file) != ".go" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		files = append(files, file)
	}
	return files
}
