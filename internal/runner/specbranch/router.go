package specbranch

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/danabrams/gromit/internal/config"
)

const defaultBranchPrefix = "gromit/spec-"

var specNamePattern = regexp.MustCompile(`^[A-Za-z0-9._/\-]+$`)

// Router maps bead labels to execution branch names.
type Router struct {
	baseBranch string
}

const specLabelPrefix = "spec:"

// NewRouter returns a Router with default branch naming rules.
func NewRouter(baseBranch string) *Router {
	baseBranch = strings.TrimSpace(baseBranch)
	if baseBranch == "" {
		baseBranch = config.DefaultBaseBranch
	}
	return &Router{baseBranch: baseBranch}
}

// BranchForLabels resolves the git branch that should be checked out for the
// provided bead labels.
func (r *Router) BranchForLabels(labels []string) (string, error) {
	rawSpecName, found := findSpecLabel(labels)
	if !found {
		return r.baseBranchOrDefault(), nil
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

// Resolve maps bead labels to the target branch.
// For beads with spec:<name> labels, it returns gromit/spec-<name>.
// For non-spec beads, it returns the base branch.
// Equivalent to BranchForLabels.
func (r *Router) Resolve(labels []string) (string, error) {
	return r.BranchForLabels(labels)
}

// HasSpecLabel reports whether labels contain at least one spec:<name> value.
func HasSpecLabel(labels []string) bool {
	_, found := findSpecLabel(labels)
	return found
}

func (r *Router) baseBranchOrDefault() string {
	if r == nil || r.baseBranch == "" {
		return config.DefaultBaseBranch
	}
	return r.baseBranch
}

func findSpecLabel(labels []string) (string, bool) {
	for _, label := range labels {
		if strings.HasPrefix(label, specLabelPrefix) {
			return strings.TrimPrefix(label, specLabelPrefix), true
		}
	}
	return "", false
}
