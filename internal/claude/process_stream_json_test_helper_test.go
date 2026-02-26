package claude

import "io"

// processStreamJSON is a test helper that wraps processStreamJSONWithCost.
func (c *Client) processStreamJSON(stdout io.Reader, output io.Writer, handler EventHandler, onToolCall ToolCallHandler) string {
	resultText, _, _, _, _ := c.processStreamJSONWithCost(stdout, output, handler, onToolCall)
	return resultText
}
