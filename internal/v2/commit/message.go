package commit

import "fmt"

// FormatMessage encodes a structured commit message.
func FormatMessage(beadID, stageName string, iteration int, decision string) string {
	return fmt.Sprintf("[bead:%s/%s/iter:%d] %s", beadID, stageName, iteration, decision)
}
