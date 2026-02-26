package main

import (
	"errors"
	"os/exec"
)

// sessionTestAgent is a test helper implementing agent.Agent interface
// with a configurable launchInDirFn for testing session launcher behavior.
type sessionTestAgent struct {
	launchInDirFn func(promptPath, dir string) error
}

func (a *sessionTestAgent) Name() string { return "session-test-agent" }

func (a *sessionTestAgent) Launch(promptPath string) error {
	return a.LaunchInDir(promptPath, "")
}

func (a *sessionTestAgent) LaunchInDir(promptPath, dir string) error {
	if a != nil && a.launchInDirFn != nil {
		return a.launchInDirFn(promptPath, dir)
	}
	return nil
}

func (a *sessionTestAgent) Command(promptPath string) (*exec.Cmd, error) {
	return nil, errors.New("not implemented")
}
