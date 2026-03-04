package tui

// HintBar returns the hint text for available actions in the current tab and state.
func HintBar(tab Tab, detailView, confirmDelete bool) string {
	switch string(tab) {
	case "backlog":
		if confirmDelete {
			return "[y/n] confirm delete"
		}
		if detailView {
			return "[q] quit | [esc] back"
		}
		return "[r] refine | [v] view | [x] delete | [q] quit"
	}
	return ""
}
