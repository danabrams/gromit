package tracker

import "testing"

func TestEncodeMetadataJSONList(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   string
		ok     bool
	}{
		{
			name:   "non-empty list",
			values: []string{"foo", "bar"},
			want:   "[\"foo\",\"bar\"]",
			ok:     true,
		},
		{
			name:   "empty list",
			values: []string{},
			want:   "",
			ok:     false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := EncodeMetadataJSONList(tt.values)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("EncodeMetadataJSONList(%v) = %q,%v, want %q,%v", tt.values, got, ok, tt.want, tt.ok)
			}
		})
	}
}
