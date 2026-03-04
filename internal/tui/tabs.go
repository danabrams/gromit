package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Tab string

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

var tabLabels = map[Tab]string{
	TabBacklog: "Backlog",
	TabSpecs:   "Specs",
	TabPlans:   "Plans",
	TabQueue:   "Queue",
	TabRunLoop: "Run loop",
}

var (
	tabBarActiveStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#82AAFF"))
	tabBarInactiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C6370")).Faint(true)
	tabBarSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C"))
)

func TabBar(activeTab Tab, width int) string {
	rendered := make([]string, len(tabOrder))
	separator := tabBarSeparatorStyle.Render(" ")
	for i, tab := range tabOrder {
		style := tabBarInactiveStyle
		if tab == activeTab {
			style = tabBarActiveStyle
		}
		rendered[i] = style.Render(tabLabel(tab))
	}

	bar := strings.Join(rendered, separator)
	if width <= 0 {
		return bar
	}

	return lipgloss.NewStyle().Width(width).Align(lipgloss.Left).Render(bar)
}

func tabLabel(tab Tab) string {
	if label, ok := tabLabels[tab]; ok {
		return label
	}
	return string(tab)
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
