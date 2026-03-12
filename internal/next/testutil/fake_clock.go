package testutil

import "time"

// FakeClock returns a configurable time and supports advancing.
type FakeClock struct {
	NowTime time.Time
}

func (c *FakeClock) Now() time.Time {
	return c.NowTime
}

func (c *FakeClock) Advance(d time.Duration) {
	c.NowTime = c.NowTime.Add(d)
}
