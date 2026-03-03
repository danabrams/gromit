package util

func CloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	clone := make([]string, len(src))
	copy(clone, src)
	return clone
}
