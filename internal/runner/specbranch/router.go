package specbranch

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	defaultMainBranch   = "main"
	defaultBranchPrefix = "gromit/spec-"
)

var specNamePattern = regexp.MustCompile(`^[A-Za-z0-9._/\-]+$`)

// Router maps bead labels to execution branch names.
type Router struct{}

const specLabelPrefix = "spec:"

// NewRouter returns a Router with default branch naming rules.
func NewRouter() *Router {
	return &Router{}
}

// BranchForLabels resolves the git branch that should be checked out for the
// provided bead labels.
func (r *Router) BranchForLabels(labels []string) (string, error) {
	rawSpecName, found := findSpecLabel(labels)
	if !found {
		return defaultMainBranch, nil
	}
	specName := strings.TrimSpace(rawSpecName)
	if specName == "" {
		return "", fmt.Errorf("invalid spec name %q", rawSpecName)
	}
	if !specNamePattern.MatchString(specName) {
		return "", fmt.Errorf("invalid spec name %q", specName)
	}
	return defaultBranchPrefix + specName, nil
}

func findSpecLabel(labels []string) (string, bool) {
	for _, label := range labels {
		if strings.HasPrefix(label, specLabelPrefix) {
			return strings.TrimPrefix(label, specLabelPrefix), true
		}
	}
	return "", false
}
