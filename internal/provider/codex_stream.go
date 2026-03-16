package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
)

const (
	codexStreamScannerMaxTokenSize = 10 * 1024 * 1024
	codexStreamWatchdogInterval    = 10 * time.Second
	highInputTokenWarningThreshold = 3000000 // 3M input tokens
)

// codexUsage represents token usage data from Codex turn.completed events.
type codexUsage struct {
	InputTokens       int     `json:"input_tokens"`
	CachedInputTokens int     `json:"cached_input_tokens"`
	OutputTokens      int     `json:"output_tokens"`
	TotalCostUSD      float64 `json:"total_cost_usd,omitempty"`
}

// codexErrorInfo represents error information from Codex turn.completed events.
type codexErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// codexItem represents an item from Codex item.started or item.completed events.
type codexItem struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Command  string `json:"command"`
	Path     string `json:"path"`
	ToolName string `json:"tool_name"`
}

// codexToolCall represents a tool call from Codex tool_call.created or tool_call.output events.
type codexToolCall struct {
	ToolName string                 `json:"tool_name"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   map[string]interface{} `json:"output,omitempty"`
}

// codexContentBlock represents content blocks in assistant-style events.
type codexContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// codexMessage represents assistant-style message payloads.
type codexMessage struct {
	Content []codexContentBlock `json:"content,omitempty"`
}

// codexDelta represents incremental text from Codex delta events.
type codexDelta struct {
	Text string `json:"text"`
}

// codexResponse represents response payloads from response.completed events.
type codexResponse struct {
	Status       string          `json:"status,omitempty"`
	Usage        *codexUsage     `json:"usage,omitempty"`
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
	InputTokens  int             `json:"input_tokens,omitempty"`
	OutputTokens int             `json:"output_tokens,omitempty"`
	ErrorInfo    *codexErrorInfo `json:"error,omitempty"`
}

// codexEvent represents a top-level Codex JSONL event.
type codexEvent struct {
	Type         string          `json:"type"`
	Text         string          `json:"text,omitempty"`
	Item         *codexItem      `json:"item,omitempty"`
	Message      *codexMessage   `json:"message,omitempty"`
	Delta        *codexDelta     `json:"delta,omitempty"`
	Response     *codexResponse  `json:"response,omitempty"`
	Status       string          `json:"status,omitempty"`
	Usage        *codexUsage     `json:"usage,omitempty"`
	Result       *codexResult    `json:"result,omitempty"`
	ToolCall     *codexToolCall  `json:"tool_call,omitempty"`
	ErrorInfo    *codexErrorInfo `json:"error,omitempty"`
	TotalCostUSD float64         `json:"total_cost_usd,omitempty"`
	InputTokens  int             `json:"input_tokens,omitempty"`
	OutputTokens int             `json:"output_tokens,omitempty"`
}

// codexResult represents nested result payloads used by some Codex event shapes.
type codexResult struct {
	Usage        *codexUsage `json:"usage,omitempty"`
	TotalCostUSD float64     `json:"total_cost_usd,omitempty"`
	InputTokens  int         `json:"input_tokens,omitempty"`
	OutputTokens int         `json:"output_tokens,omitempty"`
}

// extractAgentTextFromJSONL scans JSONL output for item.completed events
// with type "agent_message" and returns the last agent's text response.
// This is the non-streaming counterpart to processCodexStream's text extraction.
func extractAgentTextFromJSONL(output string) string {
	var last string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.Type == "item.completed" && event.Item != nil && event.Item.Type == "agent_message" {
			last = event.Item.Text
		}
	}
	return last
}

func emitStreamEvent(handler EventHandler, streamEvent map[string]interface{}) {
	if handler == nil {
		return
	}
	eventJSON, _ := json.Marshal(streamEvent)
	handler(eventJSON)
}

// emitHighInputTokenWarningIfNeeded checks if input tokens exceed the warning threshold
// and emits a warning event if they do.
func emitHighInputTokenWarningIfNeeded(handler EventHandler, usage *codexUsage) {
	if usage == nil || usage.InputTokens <= highInputTokenWarningThreshold {
		return
	}
	emitStreamEvent(handler, map[string]interface{}{
		"type":    "warning",
		"message": fmt.Sprintf("High input token usage: %d tokens exceed threshold of %d", usage.InputTokens, highInputTokenWarningThreshold),
	})
}

// processCodexStream reads Codex JSONL events from reader, converts them to StreamEvent format,
// and calls handlers for each event. Returns the final result text (from last agent_message),
// token usage data (from turn.completed), error info (from failed turn.completed), and any error encountered.
func processCodexStream(reader io.Reader, output io.Writer, handler EventHandler, toolHandler ToolCallHandler) (string, *codexUsage, *codexErrorInfo, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), codexStreamScannerMaxTokenSize)
	var lastAgentText string
	var usage *codexUsage
	var errInfo *codexErrorInfo
	var sawDeltas bool
	var eventCount int
	started := time.Now()
	lastEvent := started
	var lastEventUnixNano int64 = started.UnixNano()
	var eventsSeen int64

	stopWatchdog := make(chan struct{})
	if codexDebugEnabled() {
		go func() {
			ticker := time.NewTicker(codexStreamWatchdogInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					last := time.Unix(0, atomic.LoadInt64(&lastEventUnixNano))
					idle := time.Since(last).Round(time.Millisecond)
					codexDebugf(output, "provider debug: scanner watchdog events=%d idle=%s", atomic.LoadInt64(&eventsSeen), idle)
				case <-stopWatchdog:
					return
				}
			}
		}()
	}
	defer close(stopWatchdog)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		eventCount++
		lastEvent = time.Now()
		atomic.StoreInt64(&lastEventUnixNano, lastEvent.UnixNano())
		atomic.StoreInt64(&eventsSeen, int64(eventCount))

		var event codexEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip malformed lines silently.
			codexDebugf(output, "provider debug: malformed json event ignored len=%d", len(line))
			continue
		}
		if eventCount <= 5 || eventCount%50 == 0 {
			codexDebugf(output, "provider debug: stream event #%d type=%s", eventCount, event.Type)
		}

		// Handle different event types.
		switch event.Type {
		case "thread.started":
			emitStreamEvent(handler, map[string]interface{}{"type": "system"})
		case "message.created":
			emitStreamEvent(handler, map[string]interface{}{"type": "EventStart"})
		case "message.output_text.delta":
			if event.Text != "" {
				emitStreamEvent(handler, map[string]interface{}{
					"type": "EventContent",
					"text": event.Text,
				})
			}
		case "tool_call.created":
			var payload map[string]interface{}
			if err := json.Unmarshal(line, &payload); err != nil {
				codexDebugf(output, "provider debug: failed to parse tool_call.created payload: %v", err)
			}
			emitStreamEvent(handler, map[string]interface{}{
				"type":    "EventToolUse",
				"payload": payload,
			})

			// Invoke toolHandler if tool_call has a tool_name
			if event.ToolCall != nil && event.ToolCall.ToolName != "" && toolHandler != nil {
				toolHandler(ToolEvent{
					ToolName:  event.ToolCall.ToolName,
					Timestamp: time.Now(),
				})
			}
		case "tool_call.output":
			var payload map[string]interface{}
			if err := json.Unmarshal(line, &payload); err != nil {
				codexDebugf(output, "provider debug: failed to parse tool_call.output payload: %v", err)
			}
			emitStreamEvent(handler, map[string]interface{}{
				"type":    "EventToolResult",
				"payload": payload,
			})

			// Invoke toolHandler if tool_call has a tool_name
			if event.ToolCall != nil && event.ToolCall.ToolName != "" && toolHandler != nil {
				toolHandler(ToolEvent{
					ToolName:  event.ToolCall.ToolName,
					Timestamp: time.Now(),
				})
			}

		case "item.started":
			if event.Item != nil && toolHandler != nil {
				var toolName, filePath string
				switch event.Item.Type {
				case "command_execution":
					toolName = "Bash"
					filePath = event.Item.Command
				case "file_change":
					toolName = "Write"
					filePath = event.Item.Path
				case "mcp_tool_call":
					toolName = event.Item.ToolName
					filePath = ""
				}

				if toolName != "" {
					toolHandler(ToolEvent{
						ToolName:  toolName,
						FilePath:  filePath,
						Timestamp: time.Now(),
					})
				}
			}

		case "item.agentMessage.delta":
			// Stream incremental text to terminal in real-time.
			if event.Delta != nil && event.Delta.Text != "" {
				sawDeltas = true
				if output != nil {
					_, _ = output.Write([]byte(event.Delta.Text))
				}
			}

		case "item.completed":
			if event.Item != nil && event.Item.Type == "agent_message" {
				// Extract agent text (used as final result).
				lastAgentText = event.Item.Text

				// Write to output only if no delta events were seen
				// (deltas already streamed text incrementally).
				if !sawDeltas && output != nil && event.Item.Text != "" {
					_, _ = output.Write([]byte(event.Item.Text))
				}

				// Emit assistant event.
				emitStreamEvent(handler, map[string]interface{}{
					"type": "assistant",
					"message": map[string]interface{}{
						"content": []map[string]interface{}{
							{
								"type": "text",
								"text": event.Item.Text,
							},
						},
					},
				})
			}

		case "assistant":
			text := extractAssistantText(event.Message)
			if text == "" {
				continue
			}
			lastAgentText = text
			if output != nil {
				_, _ = output.Write([]byte(text))
			}

		case "message.output_text.done":
			if event.Text != "" {
				emitStreamEvent(handler, map[string]interface{}{
					"type": "EventContent",
					"text": event.Text,
				})
			}
			lastAgentText += event.Text
			if output != nil && event.Text != "" {
				_, _ = output.Write([]byte(event.Text))
			}

		case "result":
			// Extract usage from native result events (matches Claude's reporting path).
			// Some codex provider versions report token usage in a "result" event with
			// a nested "usage" field rather than via "turn.completed" events.
			mergedUsage, resultUsage := mergeCodexEventUsage(usage, event)
			if resultUsage != nil {
				usage = mergedUsage
				emitStreamEvent(handler, map[string]interface{}{
					"type":           "result",
					"total_cost_usd": resultUsage.TotalCostUSD,
					"input_tokens":   resultUsage.InputTokens,
					"output_tokens":  resultUsage.OutputTokens,
				})
				emitHighInputTokenWarningIfNeeded(handler, resultUsage)
			}

		case "turn.completed", "turn.failed":
			mergedUsage, turnUsage := mergeCodexEventUsage(usage, event)
			if turnUsage != nil {
				usage = mergedUsage
				if mergedUsage != nil {
					emitStreamEvent(handler, map[string]interface{}{
						"type":           "result",
						"total_cost_usd": mergedUsage.TotalCostUSD,
						"input_tokens":   mergedUsage.InputTokens,
						"output_tokens":  mergedUsage.OutputTokens,
					})
					emitStreamEvent(handler, map[string]interface{}{
						"type": "EventUsage",
						"usage": map[string]interface{}{
							"total_cost_usd": mergedUsage.TotalCostUSD,
							"input_tokens":   mergedUsage.InputTokens,
							"output_tokens":  mergedUsage.OutputTokens,
						},
					})
				}
				emitHighInputTokenWarningIfNeeded(handler, turnUsage)
			}

			// Capture error info from failed turns.
			if event.ErrorInfo != nil {
				errInfo = event.ErrorInfo
			}

			// Emit error event for failure conditions.
			if event.ErrorInfo != nil {
				emitStreamEvent(handler, map[string]interface{}{
					"type":    "error",
					"subtype": event.ErrorInfo.Type,
					"message": event.ErrorInfo.Message,
				})
			}

		case "response.completed":
			mergedUsage, responseUsage := mergeCodexEventUsage(usage, event)
			if responseUsage != nil {
				usage = mergedUsage
				emitStreamEvent(handler, map[string]interface{}{
					"type":           "result",
					"total_cost_usd": responseUsage.TotalCostUSD,
					"input_tokens":   responseUsage.InputTokens,
					"output_tokens":  responseUsage.OutputTokens,
				})
				emitHighInputTokenWarningIfNeeded(handler, responseUsage)
			}
			if event.ErrorInfo != nil {
				errInfo = event.ErrorInfo
			}
			if event.Response != nil && event.Response.ErrorInfo != nil {
				errInfo = event.Response.ErrorInfo
			}
			if errInfo != nil {
				emitStreamEvent(handler, map[string]interface{}{
					"type":    "error",
					"subtype": errInfo.Type,
					"message": errInfo.Message,
				})
			}
		case "error":
			var payload map[string]interface{}
			if err := json.Unmarshal(line, &payload); err != nil {
				codexDebugf(output, "provider debug: failed to parse error payload: %v", err)
			}
			errorInfo := map[string]interface{}{}
			if event.ErrorInfo != nil {
				errorInfo["type"] = event.ErrorInfo.Type
				errorInfo["message"] = event.ErrorInfo.Message
			}
			emitStreamEvent(handler, map[string]interface{}{
				"type":    "EventError",
				"error":   errorInfo,
				"payload": payload,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		codexDebugf(output, "provider debug: scanner err after %d events and %s since last event: %v", eventCount, time.Since(lastEvent).Round(time.Millisecond), err)
		return "", nil, nil, err
	}
	codexDebugf(output, "provider debug: scanner completed events=%d duration=%s last_event_ago=%s", eventCount, time.Since(started).Round(time.Millisecond), time.Since(lastEvent).Round(time.Millisecond))

	return lastAgentText, usage, errInfo, nil
}

func usageCost(usage *codexUsage) float64 {
	if usage == nil {
		return 0
	}
	return usage.TotalCostUSD
}

func usageInputTokens(usage *codexUsage) int {
	if usage == nil {
		return 0
	}
	return usage.InputTokens
}

func usageCachedInputTokens(usage *codexUsage) int {
	if usage == nil {
		return 0
	}
	return usage.CachedInputTokens
}

func usageOutputTokens(usage *codexUsage) int {
	if usage == nil {
		return 0
	}
	return usage.OutputTokens
}

func extractUsageFromResultEvent(event codexEvent) *codexUsage {
	var nestedUsage *codexUsage
	var nestedInputTokens, nestedOutputTokens int
	var nestedTotalCostUSD float64
	if event.Result != nil {
		nestedUsage = event.Result.Usage
		nestedInputTokens = event.Result.InputTokens
		nestedOutputTokens = event.Result.OutputTokens
		nestedTotalCostUSD = event.Result.TotalCostUSD
	}

	return extractUsageFromFields(
		event.Usage,
		event.InputTokens,
		event.OutputTokens,
		event.TotalCostUSD,
		nestedUsage,
		nestedInputTokens,
		nestedOutputTokens,
		nestedTotalCostUSD,
	)
}

func extractUsageFromTurnCompletedEvent(event codexEvent) *codexUsage {
	return extractUsageFromFields(
		event.Usage,
		event.InputTokens,
		event.OutputTokens,
		event.TotalCostUSD,
		nil,
		0,
		0,
		0,
	)
}

func extractUsageFromResponseEvent(event codexEvent) *codexUsage {
	var nestedUsage *codexUsage
	var nestedInputTokens, nestedOutputTokens int
	var nestedTotalCostUSD float64
	if event.Response != nil {
		nestedUsage = event.Response.Usage
		nestedInputTokens = event.Response.InputTokens
		nestedOutputTokens = event.Response.OutputTokens
		nestedTotalCostUSD = event.Response.TotalCostUSD
	}

	return extractUsageFromFields(
		event.Usage,
		event.InputTokens,
		event.OutputTokens,
		event.TotalCostUSD,
		nestedUsage,
		nestedInputTokens,
		nestedOutputTokens,
		nestedTotalCostUSD,
	)
}

func extractUsageFromFields(
	topLevelUsage *codexUsage,
	topLevelInputTokens, topLevelOutputTokens int,
	topLevelTotalCostUSD float64,
	nestedUsage *codexUsage,
	nestedInputTokens, nestedOutputTokens int,
	nestedTotalCostUSD float64,
) *codexUsage {
	var usage codexUsage
	found := false

	if topLevelUsage != nil {
		usage = *topLevelUsage
		found = true
	}
	found = applyUsageScalars(&usage, topLevelInputTokens, topLevelOutputTokens, topLevelTotalCostUSD) || found
	if nestedUsage != nil {
		merged := mergeCodexUsage(&usage, nestedUsage)
		if merged != nil {
			usage = *merged
		}
		found = true
	}
	found = applyUsageScalars(&usage, nestedInputTokens, nestedOutputTokens, nestedTotalCostUSD) || found
	if !found {
		return nil
	}
	return &usage
}

func extractAssistantText(message *codexMessage) string {
	if message == nil {
		return ""
	}
	var sb strings.Builder
	for _, block := range message.Content {
		if block.Type != "text" || block.Text == "" {
			continue
		}
		sb.WriteString(block.Text)
	}
	return sb.String()
}

func applyUsageScalars(usage *codexUsage, inputTokens, outputTokens int, totalCostUSD float64) bool {
	applied := false
	if inputTokens > 0 {
		usage.InputTokens = inputTokens
		applied = true
	}
	if outputTokens > 0 {
		usage.OutputTokens = outputTokens
		applied = true
	}
	if totalCostUSD > 0 {
		usage.TotalCostUSD = totalCostUSD
		applied = true
	}
	return applied
}

func mergeCodexEventUsage(existing *codexUsage, event codexEvent) (*codexUsage, *codexUsage) {
	var extracted *codexUsage
	switch event.Type {
	case "result":
		extracted = extractUsageFromResultEvent(event)
	case "turn.completed":
		extracted = extractUsageFromTurnCompletedEvent(event)
	case "response.completed":
		extracted = extractUsageFromResponseEvent(event)
	default:
		return existing, nil
	}
	if extracted == nil {
		return existing, nil
	}
	merged := mergeCodexUsage(existing, extracted)
	return merged, extracted
}

func mergeCodexUsage(existing *codexUsage, incoming *codexUsage) *codexUsage {
	if existing == nil && incoming == nil {
		return nil
	}
	if existing == nil {
		merged := *incoming
		return &merged
	}
	if incoming == nil {
		merged := *existing
		return &merged
	}

	merged := *existing
	if incoming.InputTokens > 0 {
		merged.InputTokens = incoming.InputTokens
	}
	if incoming.CachedInputTokens > 0 {
		merged.CachedInputTokens = incoming.CachedInputTokens
	}
	if incoming.OutputTokens > 0 {
		merged.OutputTokens = incoming.OutputTokens
	}
	if incoming.TotalCostUSD > 0 {
		merged.TotalCostUSD = incoming.TotalCostUSD
	}
	return &merged
}

func isCodexUsageLimitError(errInfo *codexErrorInfo) bool {
	if errInfo == nil {
		return false
	}
	return errInfo.Type == "UsageLimitExceeded"
}
