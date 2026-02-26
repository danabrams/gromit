package provider

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// GeminiStreamResult captures extracted data from a Gemini stream-json session.
type GeminiStreamResult struct {
	AssistantText string
}

// parseGeminiStream reads Gemini --output-format stream-json output line-by-line
// and collects assistant text fragments from message events.
func parseGeminiStream(reader io.Reader) (*GeminiStreamResult, error) {
	if reader == nil {
		return &GeminiStreamResult{}, nil
	}

	scanner := bufio.NewScanner(reader)
	var assistant strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		event, err := parseGeminiStreamEvent([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("parsing Gemini stream event: %w", err)
		}

		switch eventType(event) {
		case "message":
			if role, _ := event["role"].(string); role == "assistant" {
				if content, ok := event["content"].(string); ok {
					assistant.WriteString(content)
				}
			}
		case "init", "result":
			// type field drives the parsing logic; no action yet for init/result.
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning Gemini stream: %w", err)
	}

	return &GeminiStreamResult{AssistantText: assistant.String()}, nil
}

func eventType(event map[string]interface{}) string {
	v, _ := event["type"]
	s, _ := v.(string)
	return s
}
