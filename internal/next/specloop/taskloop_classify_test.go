package specloop

import "testing"

func TestIsBuildCheck(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"go build", true},
		{"go build ./...", true},
		{"go build ./internal/next/planner/...", true},
		{"go vet", true},
		{"go vet ./...", true},
		{"npm run build", true},
		{"cargo build", true},
		{"mvn compile", true},
		{"make build", true},
		{"grep -q 'func Reject' internal/next/proposaltriage/promote.go", false},
		{"grep -q '--title' cmd/gromit-next/review_proposals.go", false},
		{"awk '/stepA/{ a=NR }' file.go", false},
		{"go test -run TestFoo ./...", false},
		{"./binary --help | grep -q -- '--flag'", false},
	}
	for _, c := range cases {
		got := isBuildCheck(c.cmd)
		if got != c.want {
			t.Errorf("isBuildCheck(%q) = %v, want %v", c.cmd, got, c.want)
		}
	}
}
