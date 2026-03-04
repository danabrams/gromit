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
	case "specs":
		if confirmDelete {
			return "[y/n] confirm delete"
		}
		if detailView {
			return "[q] quit | [esc] back"
		}
		return "[p] plan | [v] view | [x] delete | [q] quit"
	case "plans":
		if confirmDelete {
			return "[y/n] confirm delete"
		}
		if detailView {
			return "[q] quit | [esc] back"
		}
		return "[d] decompose | [v] view | [x] delete | [q] quit"
	case "queue":
		if detailView {
			return "[q] quit | [esc] back"
		}
		return "[v] view | [q] quit"
	case "runloop":
		return "[q] quit"
	}
	return ""
}
