package runner

import (
	"bytes"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestGofmtCompliance(t *testing.T) {
	files := []string{
		"spc_auto_triage.go",
		"specmerge/pr_summary.go",
	}

	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Helper()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			formatted, err := format.Source(src)
			if err != nil {
				t.Fatalf("formatting %s: %v", path, err)
			}

			if !bytes.Equal(src, formatted) {
				t.Fatalf("%s is not gofmt-compliant", path)
			}
		})
	}
}

func TestRunnerCoreStdLibImportsCompact(t *testing.T) {
	files := []string{
		"runner.go",
		"orchestrator.go",
		"constructor.go",
	}
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Helper()

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			violations, err := detectStdLibImportBlankLines(path, src)
			if err != nil {
				t.Fatalf("parsing imports in %s: %v", path, err)
			}

			if len(violations) > 0 {
				t.Fatalf("stdlib imports separated by blank lines in %s:\n%s", path, strings.Join(violations, "\n"))
			}
		})
	}
}

func TestRepoGofmtCompliance(t *testing.T) {
	runRepoGofmtCheck(t)
}

func detectStdLibImportBlankLines(filePath string, src []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	type entry struct {
		path  string
		start int
		end   int
		isStd bool
	}

	var entries []entry
	for _, spec := range file.Imports {
		start := int(fset.Position(spec.Pos()).Offset)
		end := int(fset.Position(spec.End()).Offset)
		pathValue, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			pathValue = spec.Path.Value
		}
		entries = append(entries, entry{
			path:  pathValue,
			start: start,
			end:   end,
			isStd: isStdImport(pathValue),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].start < entries[j].start
	})

	var violations []string
	for i := 1; i < len(entries); i++ {
		prev := entries[i-1]
		curr := entries[i]
		if !prev.isStd || !curr.isStd || prev.end >= curr.start {
			continue
		}
		between := src[prev.end:curr.start]
		if hasBlankLine(between) {
			violations = append(violations, fmt.Sprintf("%s vs %s", prev.path, curr.path))
		}
	}
	return violations, nil
}

func isStdImport(path string) bool {
	return path != "" && !strings.Contains(path, ".")
}

func hasBlankLine(data []byte) bool {
	normalized := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	for i := 0; i < len(normalized); i++ {
		if normalized[i] != '\n' {
			continue
		}
		j := i + 1
		for j < len(normalized) && (normalized[j] == ' ' || normalized[j] == '\t' || normalized[j] == '\r') {
			j++
		}
		if j < len(normalized) && normalized[j] == '\n' {
			return true
		}
	}
	return false
}

const gofmtChunkSize = 200

func runRepoGofmtCheck(t *testing.T) {
	t.Helper()
	root := repoRootDir(t)
	files := changedGoFilesSinceBase(t, root)
	if len(files) == 0 {
		t.Skip("no changed Go files to check")
	}

	if nonCompliant := gofmtNonCompliantFiles(t, root, files); len(nonCompliant) > 0 {
		t.Fatalf("gofmt -l reported non-compliant Go files:\n%s", strings.Join(nonCompliant, "\n"))
	}
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse failed: %v", err)
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		t.Fatalf("git rev-parse returned empty root path")
	}
	return root
}

func gofmtNonCompliantFiles(t *testing.T, root string, files []string) []string {
	t.Helper()
	// Filter to only existing files (exclude deleted files that still show up in git diff)
	var existingFiles []string
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(root, f)); err == nil {
			existingFiles = append(existingFiles, f)
		}
	}

	var nonCompliant []string
	for start := 0; start < len(existingFiles); start += gofmtChunkSize {
		end := start + gofmtChunkSize
		if end > len(existingFiles) {
			end = len(existingFiles)
		}
		chunk := existingFiles[start:end]
		args := append([]string{"-l"}, chunk...)
		cmd := exec.Command("gofmt", args...)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("gofmt -l failed: %v", err)
		}
		cleaned := strings.TrimSpace(string(out))
		if cleaned == "" {
			continue
		}
		for _, line := range strings.Split(cleaned, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				nonCompliant = append(nonCompliant, trimmed)
			}
		}
	}
	return nonCompliant
}

func changedGoFilesSinceBase(t *testing.T, root string) []string {
	t.Helper()
	if base, err := gitMergeBase(root, "origin/main"); err == nil {
		return gitDiffGoFiles(t, root, base, "HEAD")
	} else {
		t.Logf("git merge-base HEAD origin/main failed: %v", err)
	}
	if parent, err := gitRevParse(root, "HEAD^"); err == nil {
		return gitDiffGoFiles(t, root, parent, "HEAD")
	} else {
		t.Logf("git rev-parse HEAD^ failed: %v", err)
	}
	t.Log("falling back to diff against HEAD")
	return gitDiffHeadGoFiles(t, root)
}

func gitDiffGoFiles(t *testing.T, root, base, head string) []string {
	t.Helper()
	return gitList(t, root, "diff", "--name-only", "--diff-filter=ACM", base, head, "--", "*.go")
}

func gitDiffHeadGoFiles(t *testing.T, root string) []string {
	t.Helper()
	return gitList(t, root, "diff", "--name-only", "--diff-filter=ACM", "HEAD", "--", "*.go")
}

func gitMergeBase(root, other string) (string, error) {
	cmd := exec.Command("git", "merge-base", "HEAD", other)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitRevParse(root, rev string) (string, error) {
	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitList(t *testing.T, root string, args ...string) []string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Logf("git %s failed: %v", strings.Join(args, " "), err)
		return nil
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	var files []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			files = append(files, trimmed)
		}
	}
	return files
}
