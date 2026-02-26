package tracker

const (
	// StatusOpen is used for tracker items that are still pending work.
	StatusOpen = "open"
	// StatusInProgress marks tracker items currently being worked.
	StatusInProgress = "in_progress"
	// StatusBlocked represents tracker items that cannot proceed.
	StatusBlocked = "blocked"
	// StatusClosed means the tracker item has finished.
	StatusClosed = "closed"
)

// Item represents a tracker entry that can be queried and managed.
type Item struct {
	ID          string
	Title       string
	Description string
	Status      string
	Metadata    map[string]string
}

// Filter narrows queries by attributes such as status, assignee, and labels.
type Filter struct {
	Statuses []string
	Assignee string
	Labels   []string
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

// UpdateRequest describes the payload needed to update a tracker item.
type UpdateRequest struct {
	ID          string
	Title       string
	Description string
	Status      string
	Metadata    map[string]string
}
