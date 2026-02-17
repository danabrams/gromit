package runtypes

const (
	maxOutputBytes         = 50 * 1024 // 50KB
	ValidationOutputHeader = "\n\n=== VALIDATION OUTPUT ===\n"
)

// TruncateOutput caps output to maxOutputBytes, keeping the tail (most recent
// content) which typically contains the most useful error information.
// Returns the input unchanged if it is within the limit.
func TruncateOutput(output string) string {
	if len(output) <= maxOutputBytes {
		return output
	}
	const marker = "...[output truncated - showing last 50KB]...\n"
	tail := output[len(output)-maxOutputBytes:]
	return marker + tail
}
