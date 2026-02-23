package worktree

import (
	"errors"
	"testing"
)

type fakeExitCodeError struct {
	msg      string
	exitCode int
}

func (e fakeExitCodeError) Error() string {
	return e.msg
}

func (e fakeExitCodeError) ExitCode() int {
	return e.exitCode
}

func TestClassifyMergeFailure(t *testing.T) {
	tests := []struct {
		name          string
		input         mergeFailureInput
		wantClass     mergeFailureClass
		wantExitCode  int
		wantExitKnown bool
	}{
		{
			name:      "conflict marker in error text",
			input:     mergeFailureInput{Err: errors.New("CONFLICT (content): merge conflict in file.txt")},
			wantClass: mergeFailureConflict,
		},
		{
			name:      "automatic merge failed in output",
			input:     mergeFailureInput{Output: "Automatic merge failed. Fix conflicts and then commit the result.", Err: errors.New("exit status 1")},
			wantClass: mergeFailureConflict,
		},
		{
			name:      "non conflict message",
			input:     mergeFailureInput{Err: errors.New("merge: gromit/review-1 - not something we can merge")},
			wantClass: mergeFailureNonConflict,
		},
		{
			name:          "captures exit code when available",
			input:         mergeFailureInput{Err: fakeExitCodeError{msg: "exit status 1", exitCode: 1}},
			wantClass:     mergeFailureNonConflict,
			wantExitCode:  1,
			wantExitKnown: true,
		},
		{
			name:          "no error has no exit code",
			input:         mergeFailureInput{},
			wantClass:     mergeFailureNonConflict,
			wantExitCode:  0,
			wantExitKnown: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyMergeFailure(tt.input)
			if got.Class != tt.wantClass {
				t.Fatalf("classifyMergeFailure() class = %v, want %v", got.Class, tt.wantClass)
			}
			if got.ExitCode != tt.wantExitCode {
				t.Fatalf("classifyMergeFailure() exit code = %d, want %d", got.ExitCode, tt.wantExitCode)
			}
			if got.ExitCodeKnown != tt.wantExitKnown {
				t.Fatalf("classifyMergeFailure() exit known = %v, want %v", got.ExitCodeKnown, tt.wantExitKnown)
			}
		})
	}
}
