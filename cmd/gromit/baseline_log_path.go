package main

import (
	"fmt"
	"path/filepath"
	"time"
)

var baselineLogPathNowFn = time.Now

func baselineLogPath(now time.Time) string {
	timestamp := now.Format("2006-01-02-150405")
	return filepath.Join("test-logs", fmt.Sprintf("refactor-baseline-%s.log", timestamp))
}

func baselineLogPathNow() string {
	return baselineLogPath(baselineLogPathNowFn())
}
