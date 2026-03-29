package main

func cloneThrashCounts(src map[string]int) map[string]int {
	if src == nil {
		return map[string]int{}
	}
	cloned := make(map[string]int, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	default:
		return 0
	}
}
