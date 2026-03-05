package dep

import (
	"strings"

	"github.com/danabrams/gromit/internal/specflow"
)

type specMetadata struct {
	ID        string
	Stage     specflow.Stage
	DependsOn []string
}

func parseSpecMetadata(specID string, frontmatter map[string]interface{}) *specMetadata {
	data := specMetadata{}
	if v, ok := frontmatter["id"].(string); ok && strings.TrimSpace(v) != "" {
		data.ID = strings.TrimSpace(v)
	} else {
		data.ID = specID
	}
	if v, ok := frontmatter["stage"].(string); ok {
		data.Stage = specflow.Stage(strings.TrimSpace(v))
	}
	data.DependsOn = parseDependsOn(frontmatter["depends_on"])
	return &data
}

func parseDependsOn(raw interface{}) []string {
	var deps []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		deps = append(deps, value)
	}

	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	case []string:
		for _, item := range v {
			add(item)
		}
	case string:
		add(v)
	}

	return deps
}
