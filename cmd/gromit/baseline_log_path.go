package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var baselineLogPathNowFn = time.Now
var baselineLogPathExistsFn = fileExists

var errBaselineLogPathCollision = errors.New("baseline log path collision")

const (
	baselineLogDir    = "test-logs"
	baselineLogPrefix = "refactor-baseline-"
	baselineLogSuffix = "-1"
)

func baselineLogPath(now time.Time) (string, error) {
	timestamp := now.Format("2006-01-02-150405")
	baseName := fmt.Sprintf("%s%s", baselineLogPrefix, timestamp)
	basePath := baselineLogPathFor(baseName, "")

	exists, err := baselineLogPathExistsFn(basePath)
	if err != nil {
		return "", err
	}
	if !exists {
		return basePath, nil
	}

	suffixedPath := baselineLogPathFor(baseName, baselineLogSuffix)
	exists, err = baselineLogPathExistsFn(suffixedPath)
	if err != nil {
		return "", err
	}
	if !exists {
		return suffixedPath, nil
	}

	return "", errBaselineLogPathCollision
}

func baselineLogPathNow() (string, error) {
	return baselineLogPath(baselineLogPathNowFn())
}

func baselineLogPathFor(baseName, suffix string) string {
	return filepath.Join(baselineLogDir, baseName+suffix+".log")
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
