package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const defaultTrendWindowSize = 30

// IterationMetric stores a single iteration with rolling-window process metrics.
type IterationMetric struct {
	Timestamp                    time.Time `json:"timestamp"`
	Iteration                    int       `json:"iteration"`
	BeadID                       string    `json:"bead_id"`
	Model                        string    `json:"model"`
	Provider                     string    `json:"provider,omitempty"`
	FailurePhase                 string    `json:"failure_phase,omitempty"`
	FailureCategory              string    `json:"failure_category,omitempty"`
	Success                      bool      `json:"success"`
	FirstPassSuccess             bool      `json:"first_pass_success"`
	Escalated                    bool      `json:"escalated"`
	DurationMs                   int64     `json:"duration_ms"`
	CostUSD                      float64   `json:"cost_usd"`
	MTTRProxyMs                  int64     `json:"mttr_proxy_ms,omitempty"`
	RollingSuccessRate           float64   `json:"rolling_success_rate"`
	RollingFailureRate           float64   `json:"rolling_failure_rate"`
	RollingFirstPassSuccess      float64   `json:"rolling_first_pass_success_rate"`
	RollingEscalationRate        float64   `json:"rolling_escalation_rate"`
	RollingAvgDurationMs         float64   `json:"rolling_avg_duration_ms"`
	RollingP95DurationMs         float64   `json:"rolling_p95_duration_ms"`
	RollingAvgCostUSD            float64   `json:"rolling_avg_cost_usd"`
	RollingAvgMTTRProxyMs        float64   `json:"rolling_avg_mttr_proxy_ms"`
	RollingPreflightFailureRate  float64   `json:"rolling_preflight_failure_rate"`
	RollingBuildFailureRate      float64   `json:"rolling_build_failure_rate"`
	RollingValidationFailureRate float64   `json:"rolling_validation_failure_rate"`
	RollingTimeoutFailureRate    float64   `json:"rolling_timeout_failure_rate"`
	FilesTouched                 int       `json:"files_touched,omitempty"`
}

// ProcessTrendWindow summarizes metrics over the latest rolling window.
type ProcessTrendWindow struct {
	SuccessRate           float64 `json:"success_rate"`
	FailureRate           float64 `json:"failure_rate"`
	FirstPassSuccess      float64 `json:"first_pass_success_rate"`
	EscalationRate        float64 `json:"escalation_rate"`
	AvgDurationMs         float64 `json:"avg_duration_ms"`
	P95DurationMs         float64 `json:"p95_duration_ms"`
	AvgCostUSD            float64 `json:"avg_cost_usd"`
	AvgMTTRProxyMs        float64 `json:"avg_mttr_proxy_ms"`
	PreflightFailureRate  float64 `json:"preflight_failure_rate"`
	BuildFailureRate      float64 `json:"build_failure_rate"`
	ValidationFailureRate float64 `json:"validation_failure_rate"`
	TimeoutFailureRate    float64 `json:"timeout_failure_rate"`
}

// TrendControlLimit captures control-limit boundaries for a process metric.
type TrendControlLimit struct {
	Metric string  `json:"metric"`
	Latest float64 `json:"latest"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	UCL    float64 `json:"ucl"`
	LCL    float64 `json:"lcl"`
}

// TrendAnomaly captures out-of-control-point anomalies in latest metrics.
type TrendAnomaly struct {
	Metric   string  `json:"metric"`
	Latest   float64 `json:"latest"`
	UCL      float64 `json:"ucl"`
	LCL      float64 `json:"lcl"`
	Severity string  `json:"severity"`
	Message  string  `json:"message"`
}

// ProcessTrend is a continuously regenerated trend snapshot.
type ProcessTrend struct {
	GeneratedAt     time.Time           `json:"generated_at"`
	TotalIterations int                 `json:"total_iterations"`
	WindowSize      int                 `json:"window_size"`
	LatestWindow    ProcessTrendWindow  `json:"latest_window"`
	ControlLimits   []TrendControlLimit `json:"control_limits"`
	Anomalies       []TrendAnomaly      `json:"anomalies"`
}

// BuildContinuousMetrics generates iteration_metrics.jsonl and process_trend.json.
func BuildContinuousMetrics(logsDir, metricsDir string, windowSize int) (*ProcessTrend, error) {
	if windowSize <= 0 {
		windowSize = defaultTrendWindowSize
	}

	entries, err := readAllIterationLogsSorted(logsDir)
	if err != nil {
		return nil, err
	}
	metrics := buildIterationMetrics(entries, windowSize)
	trend := buildProcessTrend(metrics, windowSize)

	if err := writeIterationMetrics(metricsDir, metrics); err != nil {
		return nil, err
	}
	if err := writeProcessTrend(metricsDir, trend); err != nil {
		return nil, err
	}
	return trend, nil
}

// ReadProcessTrend reads a generated process trend file.
// Returns nil,nil when the file does not exist.
func ReadProcessTrend(path string) (*ProcessTrend, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading process trend: %w", err)
	}

	var trend ProcessTrend
	if err := json.Unmarshal(data, &trend); err != nil {
		return nil, fmt.Errorf("parsing process trend: %w", err)
	}
	return &trend, nil
}

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

func readAllIterationLogsSorted(logsDir string) ([]IterationLog, error) {
	files, err := filepath.Glob(filepath.Join(logsDir, "run-*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("globbing log files: %w", err)
	}

	entries := make([]IterationLog, 0, 128)
	for _, f := range files {
		logs, err := readLogFile(f)
		if err != nil {
			continue
		}
		entries = append(entries, logs...)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Timestamp.Equal(entries[j].Timestamp) {
			if entries[i].Iteration == entries[j].Iteration {
				return entries[i].BeadID < entries[j].BeadID
			}
			return entries[i].Iteration < entries[j].Iteration
		}
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})
	return entries, nil
}

func buildIterationMetrics(entries []IterationLog, windowSize int) []IterationMetric {
	if len(entries) == 0 {
		return []IterationMetric{}
	}

	metrics := make([]IterationMetric, 0, len(entries))
	for idx, entry := range entries {
		windowStart := idx - windowSize + 1
		if windowStart < 0 {
			windowStart = 0
		}
		window := entries[windowStart : idx+1]
		w := summarizeWindow(window)

		metrics = append(metrics, IterationMetric{
			Timestamp:                    entry.Timestamp,
			Iteration:                    entry.Iteration,
			BeadID:                       entry.BeadID,
			Model:                        entry.Model,
			Provider:                     entry.Provider,
			FailurePhase:                 entry.FailurePhase,
			FailureCategory:              entry.FailureCategory,
			Success:                      entry.Success,
			FirstPassSuccess:             entry.FirstPassSuccess,
			Escalated:                    entry.Escalated,
			DurationMs:                   entry.DurationMs,
			CostUSD:                      entry.CostUSD,
			MTTRProxyMs:                  entry.MTTRProxyMs,
			FilesTouched:                 entry.FilesTouched,
			RollingSuccessRate:           w.SuccessRate,
			RollingFailureRate:           w.FailureRate,
			RollingFirstPassSuccess:      w.FirstPassSuccess,
			RollingEscalationRate:        w.EscalationRate,
			RollingAvgDurationMs:         w.AvgDurationMs,
			RollingP95DurationMs:         w.P95DurationMs,
			RollingAvgCostUSD:            w.AvgCostUSD,
			RollingAvgMTTRProxyMs:        w.AvgMTTRProxyMs,
			RollingPreflightFailureRate:  w.PreflightFailureRate,
			RollingBuildFailureRate:      w.BuildFailureRate,
			RollingValidationFailureRate: w.ValidationFailureRate,
			RollingTimeoutFailureRate:    w.TimeoutFailureRate,
		})
	}

	return metrics
}

func buildProcessTrend(metrics []IterationMetric, windowSize int) *ProcessTrend {
	trend := &ProcessTrend{
		GeneratedAt:     time.Now().UTC(),
		TotalIterations: len(metrics),
		WindowSize:      windowSize,
		LatestWindow:    ProcessTrendWindow{},
		ControlLimits:   []TrendControlLimit{},
		Anomalies:       []TrendAnomaly{},
	}
	if len(metrics) == 0 {
		return trend
	}

	latest := metrics[len(metrics)-1]
	trend.LatestWindow = ProcessTrendWindow{
		SuccessRate:      latest.RollingSuccessRate,
		FailureRate:      latest.RollingFailureRate,
		FirstPassSuccess: latest.RollingFirstPassSuccess,
		EscalationRate:   latest.RollingEscalationRate,
		AvgDurationMs:    latest.RollingAvgDurationMs,
		P95DurationMs:    latest.RollingP95DurationMs,
		AvgCostUSD:       latest.RollingAvgCostUSD,
		AvgMTTRProxyMs:   latest.RollingAvgMTTRProxyMs,
	}

	series := map[string][]float64{
		"rolling_success_rate":            extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingSuccessRate }),
		"rolling_first_pass_success_rate": extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingFirstPassSuccess }),
		"rolling_escalation_rate":         extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingEscalationRate }),
		"rolling_avg_duration_ms":         extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingAvgDurationMs }),
		"rolling_avg_cost_usd":            extractMetric(metrics, func(m IterationMetric) float64 { return m.RollingAvgCostUSD }),
	}

	for metricName, values := range series {
		limit := computeControlLimit(metricName, values)
		trend.ControlLimits = append(trend.ControlLimits, limit)
		if anomaly, ok := detectAnomaly(limit); ok {
			trend.Anomalies = append(trend.Anomalies, anomaly)
		}
	}

	sort.Slice(trend.ControlLimits, func(i, j int) bool { return trend.ControlLimits[i].Metric < trend.ControlLimits[j].Metric })
	sort.Slice(trend.Anomalies, func(i, j int) bool { return trend.Anomalies[i].Metric < trend.Anomalies[j].Metric })
	return trend
}

func extractMetric(metrics []IterationMetric, pick func(IterationMetric) float64) []float64 {
	values := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		values = append(values, pick(m))
	}
	return values
}

func computeControlLimit(metric string, values []float64) TrendControlLimit {
	latest := 0.0
	if len(values) > 0 {
		latest = values[len(values)-1]
	}
	mean, stddev := meanAndStdDev(values)
	ucl := mean + 3*stddev
	lcl := mean - 3*stddev

	if isRateMetric(metric) {
		ucl = clamp(ucl, 0, 1)
		lcl = clamp(lcl, 0, 1)
	}

	return TrendControlLimit{
		Metric: metric,
		Latest: latest,
		Mean:   mean,
		StdDev: stddev,
		UCL:    ucl,
		LCL:    lcl,
	}
}

func detectAnomaly(limit TrendControlLimit) (TrendAnomaly, bool) {
	if limit.Latest <= limit.UCL && limit.Latest >= limit.LCL {
		return TrendAnomaly{}, false
	}

	severity := "moderate"
	if limit.StdDev == 0 {
		severity = "high"
	} else {
		distance := math.Abs(limit.Latest-limit.Mean) / limit.StdDev
		if distance >= 4 {
			severity = "high"
		}
	}

	dir := "above"
	if limit.Latest < limit.LCL {
		dir = "below"
	}
	return TrendAnomaly{
		Metric:   limit.Metric,
		Latest:   limit.Latest,
		UCL:      limit.UCL,
		LCL:      limit.LCL,
		Severity: severity,
		Message:  fmt.Sprintf("latest value %.4f is %s control limits [%.4f, %.4f]", limit.Latest, dir, limit.LCL, limit.UCL),
	}, true
}

func writeIterationMetrics(metricsDir string, metrics []IterationMetric) error {
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		return fmt.Errorf("creating metrics directory: %w", err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, m := range metrics {
		if err := enc.Encode(m); err != nil {
			return fmt.Errorf("encoding iteration metric: %w", err)
		}
	}

	target := filepath.Join(metricsDir, "iteration_metrics.jsonl")
	return writeAtomic(target, buf.Bytes())
}

func writeProcessTrend(metricsDir string, trend *ProcessTrend) error {
	if trend == nil {
		return nil
	}
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		return fmt.Errorf("creating metrics directory: %w", err)
	}

	data, err := json.MarshalIndent(trend, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling process trend: %w", err)
	}

	target := filepath.Join(metricsDir, "process_trend.json")
	return writeAtomic(target, data)
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

func summarizeWindow(window []IterationLog) ProcessTrendWindow {
	if len(window) == 0 {
		return ProcessTrendWindow{}
	}

	var successes, firstPasses, escalations int
	var preflightFailures, buildFailures, validationFailures, timeoutFailures int
	var durationTotal int64
	var costTotal float64
	var mttrTotal int64
	var mttrCount int
	durations := make([]int64, 0, len(window))

	for _, e := range window {
		if e.Success {
			successes++
		}
		if e.FirstPassSuccess {
			firstPasses++
		}
		if e.Escalated {
			escalations++
		}
		switch e.FailurePhase {
		case "preflight":
			preflightFailures++
		case "build":
			buildFailures++
		case "validation":
			validationFailures++
		case "timeout":
			timeoutFailures++
		}

		durationTotal += e.DurationMs
		costTotal += e.CostUSD
		durations = append(durations, e.DurationMs)

		if e.MTTRProxyMs > 0 {
			mttrTotal += e.MTTRProxyMs
			mttrCount++
		}
	}

	count := float64(len(window))
	avgDuration := float64(durationTotal) / count
	avgCost := costTotal / count
	avgMTTR := 0.0
	if mttrCount > 0 {
		avgMTTR = float64(mttrTotal) / float64(mttrCount)
	}

	return ProcessTrendWindow{
		SuccessRate:           float64(successes) / count,
		FailureRate:           float64(len(window)-successes) / count,
		FirstPassSuccess:      float64(firstPasses) / count,
		EscalationRate:        float64(escalations) / count,
		AvgDurationMs:         avgDuration,
		P95DurationMs:         percentileInt64(durations, 95),
		AvgCostUSD:            avgCost,
		AvgMTTRProxyMs:        avgMTTR,
		PreflightFailureRate:  float64(preflightFailures) / count,
		BuildFailureRate:      float64(buildFailures) / count,
		ValidationFailureRate: float64(validationFailures) / count,
		TimeoutFailureRate:    float64(timeoutFailures) / count,
	}
}

func percentileInt64(values []int64, percentile int) float64 {
	if len(values) == 0 {
		return 0
	}
	if percentile <= 0 {
		return float64(values[0])
	}
	if percentile >= 100 {
		return float64(values[len(values)-1])
	}

	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	rank := float64(percentile) / 100.0 * float64(len(sorted)-1)
	low := int(math.Floor(rank))
	high := int(math.Ceil(rank))
	if low == high {
		return float64(sorted[low])
	}

	weight := rank - float64(low)
	return float64(sorted[low])*(1-weight) + float64(sorted[high])*weight
}

func meanAndStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	if len(values) == 1 {
		return mean, 0
	}

	var variance float64
	for _, v := range values {
		d := v - mean
		variance += d * d
	}
	variance = variance / float64(len(values))
	return mean, math.Sqrt(variance)
}

func isRateMetric(metric string) bool {
	m := strings.ToLower(metric)
	return strings.Contains(m, "rate")
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
