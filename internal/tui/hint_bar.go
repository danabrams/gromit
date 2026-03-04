package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	hintActionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#82AAFF")).Bold(true)
	hintSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C")).Faint(true)
)

var normalTabHintActions = map[Tab][]string{
	TabBacklog: {"[r] refine", "[v] view", "[x] delete"},
	TabSpecs:   {"[p] plan", "[v] view", "[x] delete"},
	TabPlans:   {"[d] decompose", "[v] view", "[x] delete"},
	TabQueue:   {"[v] view"},
	TabRunLoop: {},
}

func RenderHintBar(activeTab Tab, hasSelection bool, inDetailView bool, inConfirmation bool) string {
	if inConfirmation && tabNeedsConfirmation(activeTab) {
		return styleHint("[y/n] confirm delete")
	}
	if inDetailView {
		actions := []string{"[q] quit"}
		if activeTab != TabRunLoop {
			actions = append(actions, "[esc] back")
		}
		return renderHintActions(actions)
	}
	return renderNormalHints(activeTab, hasSelection)
}

func HintBar(tab Tab, detailView, confirmDelete bool) string {
	return RenderHintBar(tab, true, detailView, confirmDelete)
}

func renderNormalHints(tab Tab, hasSelection bool) string {
	return normalStateHintString(tab, hasSelection)
}

func normalStateHintString(tab Tab, hasSelection bool) string {
	actions, ok := normalTabHintActions[tab]
	if !ok {
		return ""
	}
	hints := []string{}
	if hasSelection && len(actions) > 0 {
		hints = append(hints, actions...)
	}
	hints = append(hints, "[q] quit")
	return renderHintActions(hints)
}

func renderHintActions(actions []string) string {
	if len(actions) == 0 {
		return ""
	}
	styled := make([]string, len(actions))
	for i, action := range actions {
		styled[i] = styleHint(action)
	}
	return strings.Join(styled, hintSeparatorStyle.Render(" | "))
}

func styleHint(action string) string {
	return hintActionStyle.Render(action)
}

func tabNeedsConfirmation(tab Tab) bool {
	switch tab {
	case TabBacklog, TabSpecs, TabPlans:
		return true
	default:
		return false
	}
}
