//go:build !linux
// +build !linux

package procutil

func collectDescendantsImpl(pid int) []int {
	return nil
}
