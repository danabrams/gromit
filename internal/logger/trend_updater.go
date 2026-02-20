package logger

import "sync"

// AsyncTrendUpdater continuously regenerates process trend metrics without blocking the run loop.
type AsyncTrendUpdater struct {
	logsDir    string
	metricsDir string
	windowSize int
	onError    func(error)

	triggerCh chan struct{}
	stopCh    chan struct{}
	wg        sync.WaitGroup
	once      sync.Once
}

// NewAsyncTrendUpdater creates and starts an asynchronous trend updater.
func NewAsyncTrendUpdater(logsDir, metricsDir string, windowSize int, onError func(error)) *AsyncTrendUpdater {
	u := &AsyncTrendUpdater{
		logsDir:    logsDir,
		metricsDir: metricsDir,
		windowSize: windowSize,
		onError:    onError,
		triggerCh:  make(chan struct{}, 1),
		stopCh:     make(chan struct{}),
	}
	u.wg.Add(1)
	go u.loop()
	return u
}

// Trigger schedules a trend refresh. Multiple rapid calls are coalesced.
func (u *AsyncTrendUpdater) Trigger() {
	if u == nil {
		return
	}
	select {
	case u.triggerCh <- struct{}{}:
	default:
	}
}

// Close stops the background worker and waits for shutdown.
func (u *AsyncTrendUpdater) Close() {
	if u == nil {
		return
	}
	u.once.Do(func() {
		close(u.stopCh)
		u.wg.Wait()
	})
}

func (u *AsyncTrendUpdater) loop() {
	defer u.wg.Done()
	for {
		select {
		case <-u.stopCh:
			return
		case <-u.triggerCh:
			u.refresh()
			u.drainPending()
		}
	}
}

func (u *AsyncTrendUpdater) drainPending() {
	for {
		select {
		case <-u.triggerCh:
			u.refresh()
		default:
			return
		}
	}
}

func (u *AsyncTrendUpdater) refresh() {
	_, err := BuildContinuousMetrics(u.logsDir, u.metricsDir, u.windowSize)
	if err != nil && u.onError != nil {
		u.onError(err)
	}
}
