package tracker

import "testing"

func TestTrackerTypeComposition(t *testing.T) {
	req := CreateRequest{
		Title:       "test item",
		Description: "describe",
		Status:      "pending",
		Metadata: map[string]string{
			"step": "initial",
		},
	}

	item := Item{
		ID:          "item-1",
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Metadata:    req.Metadata,
	}

	filter := Filter{
		Statuses: []string{"pending"},
		Assignee: "owner",
	}

	query := Query{
		Filter: filter,
		Limit:  5,
		Offset: 0,
		Sort:   "created",
	}

	if item.Title != req.Title {
		t.Fatalf("item title = %q, want %q", item.Title, req.Title)
	}

	if query.Filter.Assignee != filter.Assignee {
		t.Fatalf("query filter assignee = %q, want %q", query.Filter.Assignee, filter.Assignee)
	}
}

func TestTrackerFilterSupportsLabels(t *testing.T) {
	filter := Filter{
		Statuses: []string{"open"},
		Labels:   []string{"spec:example", "priority:high"},
	}

	query := Query{
		Filter: filter,
		Limit:  10,
	}

	if len(query.Filter.Labels) != 2 {
		t.Fatalf("query filter labels count = %d, want 2", len(query.Filter.Labels))
	}

	if query.Filter.Labels[0] != "spec:example" {
		t.Fatalf("query filter labels[0] = %q, want %q", query.Filter.Labels[0], "spec:example")
	}
}
