package tracker

import (
	"encoding/json"
	"strings"
)

func EncodeMetadataJSONList(values []string) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", false
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return "", false
	}
	return trimmed, true
}
