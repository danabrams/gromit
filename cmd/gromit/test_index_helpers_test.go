package main

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

var (
	cmdTestFuncNameRe = regexp.MustCompile(`^func (Test[[:alnum:]_]+)\(`)

	cmdTestNameIndexMu    sync.Mutex
	cmdTestNameIndexCache = make(map[string]map[string]struct{})
)

func packageTestNameIndex(t *testing.T, projectRoot, relDir string) map[string]struct{} {
	t.Helper()

	absDir := filepath.Clean(filepath.Join(projectRoot, relDir))

	cmdTestNameIndexMu.Lock()
	cached := cmdTestNameIndexCache[absDir]
	cmdTestNameIndexMu.Unlock()
	if cached != nil {
		return cached
	}

	names := make(map[string]struct{})
	err := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = f.Close()
		}()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			m := cmdTestFuncNameRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			names[m[1]] = struct{}{}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("index test names in %s: %v", relDir, err)
	}

	cmdTestNameIndexMu.Lock()
	cmdTestNameIndexCache[absDir] = names
	cmdTestNameIndexMu.Unlock()

	return names
}
