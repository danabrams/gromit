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

type tabEntry struct {
	tab    Tab
	label  string
	active bool
}

func TabBar(activeTab Tab, width int) string {
	entries := tabEntries(activeTab)
	rendered := make([]string, len(entries))
	separator := tabBarSeparatorStyle.Render(" ")
	for i, entry := range entries {
		style := tabBarInactiveStyle
		if entry.active {
			style = tabBarActiveStyle
		}
		rendered[i] = style.Render(entry.label)
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

func tabEntries(active Tab) []tabEntry {
	entries := make([]tabEntry, len(tabOrder))
	selected := active
	if !tabExists(active) && len(tabOrder) > 0 {
		selected = tabOrder[0]
	}

	for i, tab := range tabOrder {
		entries[i] = tabEntry{
			tab:    tab,
			label:  tabLabel(tab),
			active: tab == selected,
		}
	}

	return entries
}

func tabExists(tab Tab) bool {
	for _, candidate := range tabOrder {
		if candidate == tab {
			return true
		}
	}
	return false
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
