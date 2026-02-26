package tracker

import "testing"

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "open", got: StatusOpen, want: "open"},
		{name: "in_progress", got: StatusInProgress, want: "in_progress"},
		{name: "blocked", got: StatusBlocked, want: "blocked"},
		{name: "closed", got: StatusClosed, want: "closed"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}
}
