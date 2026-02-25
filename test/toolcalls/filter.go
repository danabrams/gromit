package toolcalls

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const maxCallLogLineSize = 1024 * 1024 // 1 MiB

// FilterToolCalls reads a call log file and returns all calls matching the given tool kind.
func FilterToolCalls(callLogPath string, kind ToolCallKind) ([]string, error) {
	prefix, err := ToolCallPrefix(kind)
	if err != nil {
		return nil, err
	}

	calls, err := readCallLog(callLogPath)
	if err != nil {
		return nil, err
	}

	var filtered []string
	for _, call := range calls {
		trimmed := strings.TrimSpace(call)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == prefix {
			filtered = append(filtered, trimmed)
		}
	}

	return filtered, nil
}

// readCallLog reads the call log file and returns all recorded CLI invocations.
func readCallLog(callLogPath string) ([]string, error) {
	f, err := os.Open(callLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var calls []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), maxCallLogLineSize)
	for scanner.Scan() {
		calls = append(calls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		if err == bufio.ErrTooLong {
			return nil, fmt.Errorf("call log line exceeds max size %d bytes: %w", maxCallLogLineSize, err)
		}
		return nil, err
	}

	return calls, nil
}
