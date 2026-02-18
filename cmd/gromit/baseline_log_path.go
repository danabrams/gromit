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

func baselineLogPath(now time.Time) (string, error) {
	timestamp := now.Format("2006-01-02-150405")
	baseName := fmt.Sprintf("refactor-baseline-%s", timestamp)
	basePath := filepath.Join("test-logs", baseName+".log")

	exists, err := baselineLogPathExistsFn(basePath)
	if err != nil {
		return "", err
	}
	if !exists {
		return basePath, nil
	}

	suffixedPath := filepath.Join("test-logs", baseName+"-1.log")
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
