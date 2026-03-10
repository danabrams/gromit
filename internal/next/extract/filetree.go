package extract

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/danabrams/gromit/internal/next/fact"
)

// fileInfo is serialised into each fact's Content field.
type fileInfo struct {
	Path     string `json:"path"`
	Language string `json:"language"`
	Lines    int    `json:"lines"`
}

// skipDirs lists directory names that should be skipped during the walk.
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".worktrees":   true,
}

// FileTreeExtractor walks a repository and emits one Observed fact per file.
type FileTreeExtractor struct{}

// NewFileTreeExtractor returns a ready-to-use FileTreeExtractor.
func NewFileTreeExtractor() *FileTreeExtractor {
	return &FileTreeExtractor{}
}

// Name returns the extractor identifier.
func (e *FileTreeExtractor) Name() string { return "file-tree" }

// Extract walks repoPath and returns one fact per file.
func (e *FileTreeExtractor) Extract(repoPath string) ([]fact.Fact, error) {
	var facts []fact.Fact

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}

		lines, err := countLines(path)
		if err != nil {
			return err
		}

		fi := fileInfo{
			Path:     rel,
			Language: langFromExt(filepath.Ext(rel)),
			Lines:    lines,
		}
		content, err := json.Marshal(fi)
		if err != nil {
			return err
		}

		id := "file-tree-" + strings.ReplaceAll(rel, "/", "-")
		facts = append(facts, fact.New(id, fact.Observed, string(content), "file-tree"))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return facts, nil
}

// countLines returns the number of newline-terminated lines in the file.
func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return count, nil
}

// langFromExt maps file extensions to language names.
func langFromExt(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "c++"
	case ".h", ".hpp":
		return "c-header"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".sh":
		return "shell"
	case ".sql":
		return "sql"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return "unknown"
	}
}
