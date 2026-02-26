package tracker

// Item represents a tracker entry that can be queried and managed.
type Item struct {
	ID          string
	Title       string
	Description string
	Status      string
	Metadata    map[string]string
}

// Filter narrows queries by attributes such as status or assignee.
type Filter struct {
	Statuses []string
	Assignee string
}

// Query defines pagination and sorting for tracker lookups.
type Query struct {
	Filter Filter
	Limit  int
	Offset int
	Sort   string
}

// CreateRequest describes the payload needed to create a new tracker item.
type CreateRequest struct {
	Title       string
	Description string
	Status      string
	Metadata    map[string]string
}
