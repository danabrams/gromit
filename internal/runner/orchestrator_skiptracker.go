package runner

type skipTracker struct {
	processed map[string]bool
	skipCount int
	target    int
}

func newSkipTracker() *skipTracker {
	return &skipTracker{processed: make(map[string]bool)}
}

func (s *skipTracker) hasProcessed(id string) bool {
	return s.processed[id]
}

func (s *skipTracker) markProcessed(id string) {
	s.processed[id] = true
	s.skipCount = 0
	s.target = len(s.processed)
}

func (s *skipTracker) recordSkip(_ string) bool {
	if s.target == 0 {
		return false
	}
	s.skipCount++
	return s.skipCount >= s.target
}

func (s *skipTracker) processedCount() int {
	return len(s.processed)
}

func (s *skipTracker) registerBead(id string) (skip bool, stop bool) {
	if s.hasProcessed(id) {
		return true, s.recordSkip(id)
	}
	s.markProcessed(id)
	return false, false
}
