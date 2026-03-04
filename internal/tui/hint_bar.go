package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	hintActionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#82AAFF")).Bold(true)
	hintSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C")).Faint(true)
)

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
	actions := []string{}
	switch tab {
	case TabBacklog:
		if hasSelection {
			actions = append(actions, "[r] refine", "[v] view", "[x] delete")
		}
	case TabSpecs:
		if hasSelection {
			actions = append(actions, "[p] plan", "[v] view", "[x] delete")
		}
	case TabPlans:
		if hasSelection {
			actions = append(actions, "[d] decompose", "[v] view", "[x] delete")
		}
	case TabQueue:
		if hasSelection {
			actions = append(actions, "[v] view")
		}
	case TabRunLoop:
		// run loop only supports quit; nothing to add here
	default:
		return ""
	}
	actions = append(actions, "[q] quit")
	return renderHintActions(actions)
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
