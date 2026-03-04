package tui

const (
	TabBacklog Tab = "backlog"
	TabSpecs   Tab = "specs"
	TabPlans   Tab = "plans"
	TabQueue   Tab = "queue"
	TabRunLoop Tab = "runloop"
)

var tabOrder = []Tab{
	TabBacklog,
	TabSpecs,
	TabPlans,
	TabQueue,
	TabRunLoop,
}

func nextTab(current Tab) Tab {
	if len(tabOrder) == 0 {
		return current
	}
	idx := tabIndex(current)
	return tabOrder[(idx+1)%len(tabOrder)]
}

func prevTab(current Tab) Tab {
	if len(tabOrder) == 0 {
		return current
	}
	idx := tabIndex(current)
	prev := (idx - 1 + len(tabOrder)) % len(tabOrder)
	return tabOrder[prev]
}

func tabIndex(tab Tab) int {
	for i, candidate := range tabOrder {
		if candidate == tab {
			return i
		}
	}
	return 0
}
