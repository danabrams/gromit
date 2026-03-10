package extract

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/fact"
)

// GoModExtractor parses go.mod and emits facts for the module path,
// Go version, and each direct dependency.
type GoModExtractor struct{}

// NewGoModExtractor returns a ready-to-use GoModExtractor.
func NewGoModExtractor() *GoModExtractor {
	return &GoModExtractor{}
}

// Name returns the extractor identifier.
func (e *GoModExtractor) Name() string { return "go-module" }

// Extract parses go.mod in repoPath and returns facts. If go.mod does
// not exist, it returns nil with no error.
func (e *GoModExtractor) Extract(repoPath string) ([]fact.Fact, error) {
	path := filepath.Join(repoPath, "go.mod")

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var facts []fact.Fact
	inRequireBlock := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == ")" {
			inRequireBlock = false
			continue
		}

		if strings.HasPrefix(line, "require (") || line == "require (" {
			inRequireBlock = true
			continue
		}

		if inRequireBlock {
			// Lines inside require block look like:
			//   github.com/spf13/cobra v1.8.0
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				mod := parts[0]
				ver := parts[1]
				id := "gomod-dep-" + mod
				content := fmt.Sprintf("dependency: %s %s", mod, ver)
				facts = append(facts, fact.New(id, fact.Observed, content, "go-module"))
			}
			continue
		}

		if strings.HasPrefix(line, "module ") {
			modPath := strings.TrimPrefix(line, "module ")
			facts = append(facts, fact.New("gomod-module", fact.Observed,
				"module path: "+modPath, "go-module"))
			continue
		}

		if strings.HasPrefix(line, "go ") {
			goVer := strings.TrimPrefix(line, "go ")
			facts = append(facts, fact.New("gomod-go-version", fact.Observed,
				"go version: "+goVer, "go-module"))
			continue
		}

		// Single-line require: require github.com/foo/bar v1.0.0
		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				mod := parts[1]
				ver := parts[2]
				id := "gomod-dep-" + mod
				content := fmt.Sprintf("dependency: %s %s", mod, ver)
				facts = append(facts, fact.New(id, fact.Observed, content, "go-module"))
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}
