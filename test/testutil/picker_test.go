package testutil

import "testing"

func TestPickerStdin_Refine(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		count     int
		itemIndex []int
		want      string
		wantPanic bool
	}{
		{
			name:      "select first item",
			target:    "item",
			count:     2,
			itemIndex: []int{1},
			want:      "1\n",
		},
		{
			name:      "select second item",
			target:    "item",
			count:     2,
			itemIndex: []int{2},
			want:      "2\n",
		},
		{
			name:   "select something_new with 2 items",
			target: "something_new",
			count:  2,
			want:   "3\n",
		},
		{
			name:   "select something_new with 0 items",
			target: "something_new",
			count:  0,
			want:   "1\n",
		},
		{
			name:      "item without index panics",
			target:    "item",
			count:     2,
			wantPanic: true,
		},
		{
			name:      "item with out-of-range index panics",
			target:    "item",
			count:     2,
			itemIndex: []int{3},
			wantPanic: true,
		},
		{
			name:      "unknown target panics",
			target:    "unknown",
			count:     2,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic but got none")
				} else if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			got := PickerStdin("refine", tt.target, tt.count, tt.itemIndex...)
			if !tt.wantPanic && got != tt.want {
				t.Errorf("PickerStdin(refine, %q, %d, %v) = %q, want %q", tt.target, tt.count, tt.itemIndex, got, tt.want)
			}
		})
	}
}

func TestPickerStdin_Plan(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		count     int
		itemIndex []int
		want      string
		wantPanic bool
	}{
		{
			name:      "select first spec",
			target:    "item",
			count:     3,
			itemIndex: []int{1},
			want:      "1\n",
		},
		{
			name:      "select third spec",
			target:    "item",
			count:     3,
			itemIndex: []int{3},
			want:      "3\n",
		},
		{
			name:      "item without index panics",
			target:    "item",
			count:     3,
			wantPanic: true,
		},
		{
			name:      "item with out-of-range index panics",
			target:    "item",
			count:     3,
			itemIndex: []int{4},
			wantPanic: true,
		},
		{
			name:      "unknown target panics",
			target:    "something_new",
			count:     3,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic but got none")
				} else if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			got := PickerStdin("plan", tt.target, tt.count, tt.itemIndex...)
			if !tt.wantPanic && got != tt.want {
				t.Errorf("PickerStdin(plan, %q, %d, %v) = %q, want %q", tt.target, tt.count, tt.itemIndex, got, tt.want)
			}
		})
	}
}

func TestPickerStdin_Decompose(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		count     int
		itemIndex []int
		want      string
		wantPanic bool
	}{
		{
			name:      "select first plan",
			target:    "item",
			count:     3,
			itemIndex: []int{1},
			want:      "1\n",
		},
		{
			name:      "select second plan",
			target:    "item",
			count:     3,
			itemIndex: []int{2},
			want:      "2\n",
		},
		{
			name:   "decompose_all with 2 items",
			target: "decompose_all",
			count:  2,
			want:   "3\n",
		},
		{
			name:   "decompose_all with 3 items",
			target: "decompose_all",
			count:  3,
			want:   "4\n",
		},
		{
			name:      "decompose_all with 1 item panics",
			target:    "decompose_all",
			count:     1,
			wantPanic: true,
		},
		{
			name:      "decompose_all with 0 items panics",
			target:    "decompose_all",
			count:     0,
			wantPanic: true,
		},
		{
			name:      "item without index panics",
			target:    "item",
			count:     3,
			wantPanic: true,
		},
		{
			name:      "unknown target panics",
			target:    "unknown",
			count:     3,
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.wantPanic && r == nil {
					t.Error("expected panic but got none")
				} else if !tt.wantPanic && r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			got := PickerStdin("decompose", tt.target, tt.count, tt.itemIndex...)
			if !tt.wantPanic && got != tt.want {
				t.Errorf("PickerStdin(decompose, %q, %d, %v) = %q, want %q", tt.target, tt.count, tt.itemIndex, got, tt.want)
			}
		})
	}
}

func TestPickerStdin_UnknownPickerType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown picker type but got none")
		}
	}()

	PickerStdin("unknown", "item", 1, 1)
}
