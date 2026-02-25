package toolcalls

import (
	"bufio"
	"os"
	"strings"
)

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
		if strings.HasPrefix(call, prefix) {
			filtered = append(filtered, call)
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
	for scanner.Scan() {
		calls = append(calls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return calls, nil
}
