package tui

// Keymap defines the keyboard bindings for the TUI.
type Keymap struct {
	Quit              string
	FocusNext         string
	FocusPrev         string
	ScrollUp          string
	ScrollDown        string
	SwitchView        string
	StartExplore      string
	StartRefine       string
	SendFollowUp      string
	CancelSession     string
	FocusConversation string
}

// DefaultKeymap returns the default keymap for the TUI.
func DefaultKeymap() *Keymap {
	return &Keymap{
		Quit:              "q",
		FocusNext:         "tab",
		FocusPrev:         "shift+tab",
		ScrollUp:          "up",
		ScrollDown:        "down",
		SwitchView:        "1",
		StartExplore:      "s",
		StartRefine:       "r",
		SendFollowUp:      "f",
		CancelSession:     "c",
		FocusConversation: "3",
	}
}
