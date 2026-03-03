package events

// EpilogueStartEvent is emitted when the epilogue stage begins.
type EpilogueStartEvent struct {
	BeadID    string
	Iteration int
	Success   bool // Whether build succeeded
	TimeMixin
}

func (e *EpilogueStartEvent) EventType() string {
	return "epilogue_start"
}

// EpilogueCompleteEvent is emitted when the epilogue stage completes.
// Success is false only when a lifecycle failure (close/sync) occurred.
// WarningsOccurred is true when non-fatal warnings were emitted (e.g. status
// write failure, between-iterations command failure) but the lifecycle itself
// succeeded. This separation allows consumers to distinguish fatal failures
// from non-fatal warnings.
type EpilogueCompleteEvent struct {
	BeadID           string
	Success          bool
	WarningsOccurred bool
	TimeMixin
}

func (e *EpilogueCompleteEvent) EventType() string {
	return "epilogue_complete"
}

// BeadCloseEvent is emitted when a bead is closed successfully.
type BeadCloseEvent struct {
	BeadID string
	TimeMixin
}

func (e *BeadCloseEvent) EventType() string {
	return "bead_close"
}

// BeadCleanupEvent is emitted when epilogue cleanup operations complete.
type BeadCleanupEvent struct {
	BeadID string
	Action string // e.g., "sync", "merge", "worktree_cleanup"
	TimeMixin
}

func (e *BeadCleanupEvent) EventType() string {
	return "bead_cleanup"
}
