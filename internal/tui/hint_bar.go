package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	hintActionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#82AAFF")).Bold(true)
	hintSeparatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C")).Faint(true)
)

var normalTabBaseActions = map[Tab][]string{
	TabBacklog: {"[r] refine", "[v] view", "[x] delete"},
	TabSpecs:   {"[p] plan", "[v] view", "[x] delete"},
	TabPlans:   {"[d] decompose", "[v] view", "[x] delete"},
	TabQueue:   {"[v] view"},
	TabRunLoop: {},
}

func RenderHintBar(activeTab Tab, hasSelection bool, inDetailView bool, inConfirmation bool) string {
	if inConfirmation {
		if actions := confirmationHintActions(activeTab); len(actions) > 0 {
			return renderHintActions(actions)
		}
	}
	if inDetailView {
		return renderHintActions(detailViewHintActions(activeTab))
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
	return renderHintActions(normalHintActions(tab, hasSelection))
}

func normalHintActions(tab Tab, hasSelection bool) []string {
	actions := []string{}
	if hasSelection {
		actions = append(actions, perTabNormalHintActions(tab)...)
	}
	return append(actions, "[q] quit")
}

func perTabNormalHintActions(tab Tab) []string {
	if tabActions, ok := normalTabBaseActions[tab]; ok {
		copied := make([]string, len(tabActions))
		copy(copied, tabActions)
		return copied
	}
	return nil
}

func detailViewHintActions(tab Tab) []string {
	if tab == TabRunLoop {
		return []string{"[q] quit"}
	}
	return []string{"[q] quit", "[esc] back"}
}

func confirmationHintActions(tab Tab) []string {
	if !tabNeedsConfirmation(tab) {
		return []string{}
	}
	return []string{"[y/n] confirm delete"}
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
