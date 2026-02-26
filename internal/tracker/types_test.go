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
