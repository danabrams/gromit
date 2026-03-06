package dep

import (
	"strings"
)

type specMetadata struct {
	ID        string
	Accepted  bool
	DependsOn []string
}

func parseSpecMetadata(specID string, frontmatter map[string]interface{}) *specMetadata {
	data := specMetadata{}
	if v, ok := frontmatter["id"].(string); ok && strings.TrimSpace(v) != "" {
		data.ID = strings.TrimSpace(v)
	} else {
		data.ID = specID
	}
	data.Accepted = parseAccepted(frontmatter["accepted"])
	data.DependsOn = parseDependsOn(frontmatter["depends_on"])
	return &data
}

func parseAccepted(raw interface{}) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		lower := strings.TrimSpace(strings.ToLower(v))
		return lower == "true" || lower == "yes" || lower == "1"
	}
	return false
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
