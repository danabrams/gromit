package visionmetrics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

const fileMode = 0644

// LoadRecords reads a JSONL file and returns the list of Records.
func LoadRecords(path string) ([]Record, error) {
	records := make([]Record, 0)
	err := withFileLock(path, func() error {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var record Record
			if err := json.Unmarshal(line, &record); err != nil {
				return err
			}
			records = append(records, record)
		}

		return scanner.Err()
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

// withFileLock acquires an exclusive advisory lock adjacent to the target path.
func withFileLock(path string, fn func() error) error {
	lockPath := path + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, fileMode)
	if err != nil {
		return fmt.Errorf("opening lock file: %w", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquiring file lock: %w", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	return fn()
}

// AppendRecord appends a Record to a JSONL file.
func AppendRecord(path string, record Record) error {
	return withFileLock(path, func() error {
		// Open file for appending, creating if it doesn't exist
		file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
		if err != nil {
			return err
		}
		defer file.Close()

		// Marshal record to JSON and write as a line
		data, err := json.Marshal(record)
		if err != nil {
			return err
		}

		// Write JSON and newline in a single WriteString call for atomicity
		_, err = file.WriteString(string(data) + "\n")
		return err
	})
}
