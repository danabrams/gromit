package prompt

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var h2HeaderPattern = regexp.MustCompile(`(?m)^##\s+([^\n#][^\n]*)\s*$`)
var architectureBulletPattern = regexp.MustCompile("(?m)^\\s*-\\s*`([^`]+)`\\s*(?:—|-)\\s*(.+?)\\s*$")

// ScopedArchitectureEntry describes one architecture bullet line.
type ScopedArchitectureEntry struct {
	Path        string
	Description string
}

type claudeSections struct {
	Preamble             string
	ArchitectureBody     string
	KeyPrinciplesSection string
	HasArchitecture      bool
	HasKeyPrinciples     bool
}

const scopedDescriptionFallback = "Task-relevant package context."

// parseClaudeSections extracts top-level CLAUDE.md sections needed for scoped rendering.
func parseClaudeSections(content string) claudeSections {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	matches := h2HeaderPattern.FindAllStringSubmatchIndex(normalized, -1)
	if len(matches) == 0 {
		return claudeSections{Preamble: strings.TrimSpace(normalized)}
	}

	sections := claudeSections{
		Preamble: strings.TrimSpace(normalized[:matches[0][0]]),
	}

	for i, match := range matches {
		sectionStart := match[0]
		sectionHeaderEnd := match[1]
		nameStart := match[2]
		nameEnd := match[3]

		sectionEnd := len(normalized)
		if i+1 < len(matches) {
			sectionEnd = matches[i+1][0]
		}

		sectionName := strings.TrimSpace(strings.ToLower(normalized[nameStart:nameEnd]))
		sectionBody := strings.Trim(normalized[sectionHeaderEnd:sectionEnd], "\n")
		sectionRaw := strings.Trim(normalized[sectionStart:sectionEnd], "\n")

		switch sectionName {
		case "architecture":
			sections.ArchitectureBody = sectionBody
			sections.HasArchitecture = true
		case "key principles":
			sections.KeyPrinciplesSection = sectionRaw
			sections.HasKeyPrinciples = true
		}
	}

	return sections
}

// renderScopedArchitectureSection renders a minimal Architecture section from scoped package entries.
func renderScopedArchitectureSection(entries []ScopedArchitectureEntry) string {
	bullets := renderScopedArchitectureBullets(entries)
	if len(bullets) == 0 {
		return "## Architecture"
	}

	return "## Architecture\n\n" + strings.Join(bullets, "\n")
}

func renderScopedArchitectureBullets(entries []ScopedArchitectureEntry) []string {
	if len(entries) == 0 {
		return []string{}
	}

	normalized := make(map[string]string, len(entries))
	for _, entry := range entries {
		path := normalizeScopedPath(entry.Path)
		if path == "" {
			continue
		}

		description := strings.TrimSpace(entry.Description)
		if description == "" {
			description = scopedDescriptionFallback
		}

		if _, exists := normalized[path]; !exists {
			normalized[path] = description
		}
	}

	paths := make([]string, 0, len(normalized))
	for path := range normalized {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	bullets := make([]string, 0, len(paths))
	for _, path := range paths {
		bullets = append(bullets, "- `"+path+"` — "+normalized[path])
	}

	return bullets
}

// resolveScopedArchitectureEntries resolves one-line descriptions for package paths with
// priority CLAUDE architecture bullets -> Go package docs -> generic fallback.
func resolveScopedArchitectureEntries(paths []string, architectureBody string, repoRoot string) []ScopedArchitectureEntry {
	if len(paths) == 0 {
		return []ScopedArchitectureEntry{}
	}

	descriptions := parseArchitectureBulletDescriptions(architectureBody)
	normalized := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		normalizedPath := normalizeScopedPath(path)
		if normalizedPath == "" {
			continue
		}
		normalized[normalizedPath] = struct{}{}
	}

	sortedPaths := make([]string, 0, len(normalized))
	for path := range normalized {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	entries := make([]ScopedArchitectureEntry, 0, len(sortedPaths))
	for _, path := range sortedPaths {
		entries = append(entries, ScopedArchitectureEntry{
			Path:        path,
			Description: resolveScopedPackageDescription(path, descriptions, repoRoot),
		})
	}

	return entries
}

func resolveScopedPackageDescription(path string, architectureDescriptions map[string]string, repoRoot string) string {
	if description, ok := findArchitectureDescription(path, architectureDescriptions); ok {
		return description
	}

	if description := readPackageDocDescription(path, repoRoot); description != "" {
		return description
	}

	return scopedDescriptionFallback
}

func parseArchitectureBulletDescriptions(architectureBody string) map[string]string {
	matches := architectureBulletPattern.FindAllStringSubmatch(architectureBody, -1)
	descriptions := make(map[string]string, len(matches))
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		path := normalizeScopedPath(match[1])
		description := strings.TrimSpace(match[2])
		if path == "" || description == "" {
			continue
		}

		if _, exists := descriptions[path]; !exists {
			descriptions[path] = description
		}
	}
	return descriptions
}

func findArchitectureDescription(path string, descriptions map[string]string) (string, bool) {
	if len(descriptions) == 0 {
		return "", false
	}

	normalizedPath := normalizeScopedPath(path)
	if normalizedPath == "" {
		return "", false
	}

	candidates := []string{normalizedPath}
	if strings.HasPrefix(normalizedPath, "internal/") {
		candidates = append(candidates, strings.TrimPrefix(normalizedPath, "internal/"))
	}

	for _, candidate := range candidates {
		description, ok := descriptions[candidate]
		if ok && strings.TrimSpace(description) != "" {
			return description, true
		}
	}

	return "", false
}

func readPackageDocDescription(path string, repoRoot string) string {
	normalizedPath := normalizeScopedPath(path)
	normalizedPath = strings.TrimSuffix(normalizedPath, "/")
	if normalizedPath == "" || strings.TrimSpace(repoRoot) == "" {
		return ""
	}

	packageDir := filepath.Join(repoRoot, filepath.FromSlash(normalizedPath))
	parsed, err := parser.ParseDir(token.NewFileSet(), packageDir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.ParseComments)
	if err != nil || len(parsed) == 0 {
		return ""
	}

	pkgNames := make([]string, 0, len(parsed))
	for name := range parsed {
		pkgNames = append(pkgNames, name)
	}
	sort.Strings(pkgNames)

	for _, pkgName := range pkgNames {
		description := readPackageSynopsis(parsed[pkgName])
		if description != "" {
			return description
		}
	}

	return ""
}

func readPackageSynopsis(pkg *ast.Package) string {
	if pkg == nil {
		return ""
	}

	if file, ok := pkg.Files["doc.go"]; ok && file != nil {
		if synopsis := commentSynopsis(file.Doc); synopsis != "" {
			return synopsis
		}
	}

	fileNames := make([]string, 0, len(pkg.Files))
	for name := range pkg.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)

	for _, fileName := range fileNames {
		file := pkg.Files[fileName]
		if file == nil {
			continue
		}

		if synopsis := commentSynopsis(file.Doc); synopsis != "" {
			return synopsis
		}
	}

	return ""
}

func commentSynopsis(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}

	return strings.TrimSpace(doc.Synopsis(group.Text()))
}

func normalizeScopedPath(path string) string {
	normalized := strings.TrimSpace(path)
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return ""
	}
	return normalized + "/"
}

// renderScopedClaudeContent replaces only the Architecture section while preserving
// Key Principles verbatim. If required sections are missing, it returns the input unchanged.
func renderScopedClaudeContent(content string, entries []ScopedArchitectureEntry) string {
	sections := parseClaudeSections(content)
	if !sections.HasArchitecture || !sections.HasKeyPrinciples {
		return content
	}

	parts := make([]string, 0, 3)
	if strings.TrimSpace(sections.Preamble) != "" {
		parts = append(parts, sections.Preamble)
	}
	parts = append(parts, renderScopedArchitectureSection(entries), sections.KeyPrinciplesSection)

	return strings.Join(parts, "\n\n")
}
