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
	"time"

	"github.com/danabrams/gromit/internal/failurephase"
	"github.com/danabrams/gromit/internal/prompt"
)

const (
	defaultTrendWindowSize         = 30
	p95Percentile                  = 95
	promptSectionTopLimit          = 10
	controlLimitSigmaMultiplier    = 3
	ewmaLambda                     = 0.15
	ewmaSigmaMultiplier            = 2.7
	nelsonRule2MinRunLength        = 9
	nelsonRule2Name                = "nelson_rule_2"
	highSeveritySigmaThreshold     = 4
	anomalySeverityModerate        = "moderate"
	anomalySeverityHigh            = "high"
	anomalyDirectionAbove          = "above"
	anomalyDirectionBelow          = "below"
	iterationMetricsFilename       = "iteration_metrics.jsonl"
	processTrendFilename           = "process_trend.json"
	metricRollingSuccessRate       = "rolling_success_rate"
	metricRollingFirstPassSuccess  = "rolling_first_pass_success_rate"
	metricRollingEscalationRate    = "rolling_escalation_rate"
	metricRollingAvgDurationMs     = "rolling_avg_duration_ms"
	metricRollingAvgValidationMs   = "rolling_avg_validation_ms"
	metricRollingAvgCostUSD        = "rolling_avg_cost_usd"
	metricRollingPreflightFailure  = "rolling_preflight_failure_rate"
	metricRollingBuildFailure      = "rolling_build_failure_rate"
	metricRollingValidationFailure = "rolling_validation_failure_rate"
	metricRollingTimeoutFailure    = "rolling_timeout_failure_rate"
	metricRollingAvgCostPerBeadUSD = "rolling_avg_cost_per_bead_usd"
	metricEWMASuccessRate          = "ewma_success_rate"
	metricEWMACostUSD              = "ewma_cost_usd"
	metricEWMADurationMs           = "ewma_duration_ms"
	transportDisconnectFailure     = "transport_disconnect"
)

var phaseRateMetrics = []string{
	metricRollingPreflightFailure,
	metricRollingBuildFailure,
	metricRollingValidationFailure,
	metricRollingTimeoutFailure,
}

type metricSeriesDefinition struct {
	// latestSample is the most recent rolling value shown in process_trend.json.
	name         string
	latestSample func(IterationMetric) float64
	// historySample is the per-iteration series used to compute mean/stddev control limits.
	historySample func(IterationMetric) float64
}

var trendControlLimitSeries = []metricSeriesDefinition{
	{
		name:          metricRollingSuccessRate,
		latestSample:  func(m IterationMetric) float64 { return m.RollingSuccessRate },
		historySample: func(m IterationMetric) float64 { return boolToFloat64(m.Success) },
	},
	{
		name:          metricRollingFirstPassSuccess,
		latestSample:  func(m IterationMetric) float64 { return m.RollingFirstPassSuccess },
		historySample: func(m IterationMetric) float64 { return m.RollingFirstPassSuccess },
	},
	{
		name:          metricRollingEscalationRate,
		latestSample:  func(m IterationMetric) float64 { return m.RollingEscalationRate },
		historySample: func(m IterationMetric) float64 { return m.RollingEscalationRate },
	},
	{
		name:          metricRollingAvgDurationMs,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgDurationMs },
		historySample: func(m IterationMetric) float64 { return float64(m.DurationMs) },
	},
	{
		name:          metricRollingAvgValidationMs,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgValidationMs },
		historySample: func(m IterationMetric) float64 { return m.RollingAvgValidationMs },
	},
	{
		name:          metricRollingAvgCostUSD,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgCostUSD },
		historySample: func(m IterationMetric) float64 { return m.CostUSD },
	},
	{
		name:          metricRollingAvgCostPerBeadUSD,
		latestSample:  func(m IterationMetric) float64 { return m.RollingAvgCostPerBeadUSD },
		historySample: func(m IterationMetric) float64 { return m.RollingAvgCostPerBeadUSD },
	},
	{
		name:          metricRollingPreflightFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingPreflightFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingPreflightFailureRate },
	},
	{
		name:          metricRollingBuildFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingBuildFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingBuildFailureRate },
	},
	{
		name:          metricRollingValidationFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingValidationFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingValidationFailureRate },
	},
	{
		name:          metricRollingTimeoutFailure,
		latestSample:  func(m IterationMetric) float64 { return m.RollingTimeoutFailureRate },
		historySample: func(m IterationMetric) float64 { return m.RollingTimeoutFailureRate },
	},
}

// IterationMetric stores a single iteration with rolling-window process metrics.
type IterationMetric struct {
	Timestamp                    time.Time                 `json:"timestamp"`
	Iteration                    int                       `json:"iteration"`
	BeadID                       string                    `json:"bead_id"`
	Model                        string                    `json:"model"`
	Provider                     string                    `json:"provider,omitempty"`
	FailurePhase                 string                    `json:"failure_phase,omitempty"`
	FailureCategory              string                    `json:"failure_category,omitempty"`
	Success                      bool                      `json:"success"`
	FirstPassSuccess             bool                      `json:"first_pass_success"`
	Escalated                    bool                      `json:"escalated"`
	DurationMs                   int64                     `json:"duration_ms"`
	ValidationDurationMs         int64                     `json:"validation_duration_ms,omitempty"`
	CostUSD                      float64                   `json:"cost_usd"`
	InputTokens                  int                       `json:"input_tokens,omitempty"`
	OutputTokens                 int                       `json:"output_tokens,omitempty"`
	MTTRProxyMs                  int64                     `json:"mttr_proxy_ms,omitempty"`
	RollingSuccessRate           float64                   `json:"rolling_success_rate"`
	RollingFailureRate           float64                   `json:"rolling_failure_rate"`
	RollingFirstPassSuccess      float64                   `json:"rolling_first_pass_success_rate"`
	RollingEscalationRate        float64                   `json:"rolling_escalation_rate"`
	RollingAvgDurationMs         float64                   `json:"rolling_avg_duration_ms"`
	RollingP95DurationMs         float64                   `json:"rolling_p95_duration_ms"`
	RollingAvgValidationMs       float64                   `json:"rolling_avg_validation_ms"`
	RollingP95ValidationMs       float64                   `json:"rolling_p95_validation_ms"`
	RollingAvgCostUSD            float64                   `json:"rolling_avg_cost_usd"`
	RollingAvgCostPerBeadUSD     float64                   `json:"rolling_avg_cost_per_bead_usd"`
	RollingAvgMTTRProxyMs        float64                   `json:"rolling_avg_mttr_proxy_ms"`
	RollingPreflightFailureRate  float64                   `json:"rolling_preflight_failure_rate"`
	RollingBuildFailureRate      float64                   `json:"rolling_build_failure_rate"`
	RollingValidationFailureRate float64                   `json:"rolling_validation_failure_rate"`
	RollingTimeoutFailureRate    float64                   `json:"rolling_timeout_failure_rate"`
	EWMASuccessRate              EWMAMetricState           `json:"ewma_success_rate"`
	EWMACostUSD                  EWMAMetricState           `json:"ewma_cost_usd"`
	EWMADurationMs               EWMAMetricState           `json:"ewma_duration_ms"`
	FilesTouched                 int                       `json:"files_touched,omitempty"`
	PromptDiagnostics            *prompt.PromptDiagnostics `json:"prompt_diagnostics,omitempty"`
}

// EWMAMetricState captures exponential moving-average state for one metric.
type EWMAMetricState struct {
	Lambda float64 `json:"lambda"`
	L      float64 `json:"l"`
	Value  float64 `json:"value"`
	Z      float64 `json:"z"`
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	UCL    float64 `json:"ucl"`
	LCL    float64 `json:"lcl"`
}

// PromptTypeSummary captures aggregate token usage for one prompt type.
type PromptTypeSummary struct {
	PromptType         string  `json:"prompt_type"`
	InvocationCount    int     `json:"invocation_count"`
	AvgEstimatedTokens float64 `json:"avg_estimated_tokens"`
}

// PromptSectionSummary captures aggregate token usage for one prompt section.
type PromptSectionSummary struct {
	Section         string `json:"section"`
	EstimatedTokens int    `json:"estimated_tokens"`
}

// ReconciliationDrift captures estimate-vs-reported token drift distribution.
type ReconciliationDrift struct {
	SampleCount          int     `json:"sample_count"`
	MeanAbsTokenDeltaPct float64 `json:"mean_abs_token_delta_pct"`
	P95AbsTokenDeltaPct  float64 `json:"p95_abs_token_delta_pct"`
}

// PromptTokenSummary captures prompt-level token accounting metrics.
type PromptTokenSummary struct {
	ByPromptType          []PromptTypeSummary    `json:"by_prompt_type"`
	BySectionTop10        []PromptSectionSummary `json:"by_section_top_10"`
	BudgetActionFrequency map[string]int         `json:"budget_action_frequency"`
	ReconciliationDrift   ReconciliationDrift    `json:"reconciliation_drift"`
}

// ProcessTrendWindow summarizes metrics over the latest rolling window.
type ProcessTrendWindow struct {
	SuccessRate           float64 `json:"success_rate"`
	FailureRate           float64 `json:"failure_rate"`
	FirstPassSuccess      float64 `json:"first_pass_success_rate"`
	EscalationRate        float64 `json:"escalation_rate"`
	AvgDurationMs         float64 `json:"avg_duration_ms"`
	P95DurationMs         float64 `json:"p95_duration_ms"`
	AvgValidationMs       float64 `json:"avg_validation_ms"`
	P95ValidationMs       float64 `json:"p95_validation_ms"`
	AvgCostUSD            float64 `json:"avg_cost_usd"`
	AvgCostPerBeadUSD     float64 `json:"avg_cost_per_bead_usd"`
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

// PatternViolation captures Nelson-rule pattern signals.
type PatternViolation struct {
	Metric     string  `json:"metric"`
	Rule       string  `json:"rule"`
	Direction  string  `json:"direction"`
	RunLength  int     `json:"run_length"`
	CenterLine float64 `json:"center_line"`
	Message    string  `json:"message"`
}

// ProviderMetrics captures provider-level aggregates from iteration metrics.
type ProviderMetrics struct {
	Name                 string  `json:"name"`
	TotalInvocations     int     `json:"total_invocations"`
	Successes            int     `json:"successes"`
	SuccessRate          float64 `json:"success_rate"`
	TransportFailures    int     `json:"transport_failures"`
	TransportFailureRate float64 `json:"transport_failure_rate"`
	FallbacksTriggered   int     `json:"fallbacks_triggered"`
	AvgDurationMs        float64 `json:"avg_duration_ms"`
	TotalCostUSD         float64 `json:"total_cost_usd"`
	TotalInputTokens     int     `json:"total_input_tokens"`
	TotalOutputTokens    int     `json:"total_output_tokens"`
}

// ProcessTrend is a continuously regenerated trend snapshot.
type ProcessTrend struct {
	GeneratedAt        time.Time           `json:"generated_at"`
	TotalIterations    int                 `json:"total_iterations"`
	WindowSize         int                 `json:"window_size"`
	LatestWindow       ProcessTrendWindow  `json:"latest_window"`
	PromptTokenSummary PromptTokenSummary  `json:"prompt_token_summary"`
	ProviderMetrics    []ProviderMetrics   `json:"provider_metrics"`
	ControlLimits      []TrendControlLimit `json:"control_limits"`
	Anomalies          []TrendAnomaly      `json:"anomalies"`
	EWMAAnomalies      []TrendAnomaly      `json:"ewma_anomalies"`
	PatternViolations  []PatternViolation  `json:"pattern_violations"`
}

func newPromptTokenSummary() PromptTokenSummary {
	return PromptTokenSummary{
		ByPromptType:          []PromptTypeSummary{},
		BySectionTop10:        []PromptSectionSummary{},
		BudgetActionFrequency: map[string]int{},
	}
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
	trend.normalizeNilFields()
	return &trend, nil
}

func (t *ProcessTrend) normalizeNilFields() {
	if t == nil {
		return
	}
	if t.ProviderMetrics == nil {
		t.ProviderMetrics = []ProviderMetrics{}
	}
	if t.ControlLimits == nil {
		t.ControlLimits = []TrendControlLimit{}
	}
	if t.Anomalies == nil {
		t.Anomalies = []TrendAnomaly{}
	}
	if t.EWMAAnomalies == nil {
		t.EWMAAnomalies = []TrendAnomaly{}
	}
	if t.PatternViolations == nil {
		t.PatternViolations = []PatternViolation{}
	}
	if t.PromptTokenSummary.ByPromptType == nil {
		t.PromptTokenSummary.ByPromptType = []PromptTypeSummary{}
	}
	if t.PromptTokenSummary.BySectionTop10 == nil {
		t.PromptTokenSummary.BySectionTop10 = []PromptSectionSummary{}
	}
	if t.PromptTokenSummary.BudgetActionFrequency == nil {
		t.PromptTokenSummary.BudgetActionFrequency = map[string]int{}
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
	successValues := make([]float64, 0, len(entries))
	costValues := make([]float64, 0, len(entries))
	durationValues := make([]float64, 0, len(entries))
	var (
		prevSuccessZ  float64
		prevCostZ     float64
		prevDurationZ float64
		hasPrevious   bool
	)
	for idx, entry := range entries {
		windowStart := idx - windowSize + 1
		if windowStart < 0 {
			windowStart = 0
		}
		window := entries[windowStart : idx+1]
		w := summarizeWindow(window)
		successValue := boolToFloat64(entry.Success)
		costValue := entry.CostUSD
		durationValue := float64(entry.DurationMs)
		successValues = append(successValues, successValue)
		costValues = append(costValues, costValue)
		durationValues = append(durationValues, durationValue)

		successEWMA := computeEWMAState(metricEWMASuccessRate, successValue, successValues, prevSuccessZ, hasPrevious)
		costEWMA := computeEWMAState(metricEWMACostUSD, costValue, costValues, prevCostZ, hasPrevious)
		durationEWMA := computeEWMAState(metricEWMADurationMs, durationValue, durationValues, prevDurationZ, hasPrevious)
		prevSuccessZ = successEWMA.Z
		prevCostZ = costEWMA.Z
		prevDurationZ = durationEWMA.Z
		hasPrevious = true

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
			ValidationDurationMs:         entry.ValidationDurationMs,
			CostUSD:                      entry.CostUSD,
			InputTokens:                  entry.InputTokens,
			OutputTokens:                 entry.OutputTokens,
			MTTRProxyMs:                  entry.MTTRProxyMs,
			FilesTouched:                 entry.FilesTouched,
			PromptDiagnostics:            entry.PromptDiagnostics,
			RollingSuccessRate:           w.SuccessRate,
			RollingFailureRate:           w.FailureRate,
			RollingFirstPassSuccess:      w.FirstPassSuccess,
			RollingEscalationRate:        w.EscalationRate,
			RollingAvgDurationMs:         w.AvgDurationMs,
			RollingP95DurationMs:         w.P95DurationMs,
			RollingAvgValidationMs:       w.AvgValidationMs,
			RollingP95ValidationMs:       w.P95ValidationMs,
			RollingAvgCostUSD:            w.AvgCostUSD,
			RollingAvgCostPerBeadUSD:     w.AvgCostPerBeadUSD,
			RollingAvgMTTRProxyMs:        w.AvgMTTRProxyMs,
			RollingPreflightFailureRate:  w.PreflightFailureRate,
			RollingBuildFailureRate:      w.BuildFailureRate,
			RollingValidationFailureRate: w.ValidationFailureRate,
			RollingTimeoutFailureRate:    w.TimeoutFailureRate,
			EWMASuccessRate:              successEWMA,
			EWMACostUSD:                  costEWMA,
			EWMADurationMs:               durationEWMA,
		})
	}

	return metrics
}

func buildProcessTrend(metrics []IterationMetric, windowSize int) *ProcessTrend {
	trend := &ProcessTrend{
		GeneratedAt:        time.Now().UTC(),
		TotalIterations:    len(metrics),
		WindowSize:         windowSize,
		LatestWindow:       ProcessTrendWindow{},
		PromptTokenSummary: newPromptTokenSummary(),
		ProviderMetrics:    []ProviderMetrics{},
		ControlLimits:      []TrendControlLimit{},
		Anomalies:          []TrendAnomaly{},
		EWMAAnomalies:      []TrendAnomaly{},
		PatternViolations:  []PatternViolation{},
	}
	if len(metrics) == 0 {
		return trend
	}

	latestMetric := metrics[len(metrics)-1]
	trend.LatestWindow = ProcessTrendWindow{
		SuccessRate:           latestMetric.RollingSuccessRate,
		FailureRate:           latestMetric.RollingFailureRate,
		FirstPassSuccess:      latestMetric.RollingFirstPassSuccess,
		EscalationRate:        latestMetric.RollingEscalationRate,
		AvgDurationMs:         latestMetric.RollingAvgDurationMs,
		P95DurationMs:         latestMetric.RollingP95DurationMs,
		AvgValidationMs:       latestMetric.RollingAvgValidationMs,
		P95ValidationMs:       latestMetric.RollingP95ValidationMs,
		AvgCostUSD:            latestMetric.RollingAvgCostUSD,
		AvgCostPerBeadUSD:     latestMetric.RollingAvgCostPerBeadUSD,
		AvgMTTRProxyMs:        latestMetric.RollingAvgMTTRProxyMs,
		PreflightFailureRate:  latestMetric.RollingPreflightFailureRate,
		BuildFailureRate:      latestMetric.RollingBuildFailureRate,
		ValidationFailureRate: latestMetric.RollingValidationFailureRate,
		TimeoutFailureRate:    latestMetric.RollingTimeoutFailureRate,
	}
	trend.PromptTokenSummary = summarizePromptTokens(metrics, windowSize)
	trend.ProviderMetrics = computeProviderMetrics(metrics)

	for _, metric := range trendControlLimitSeries {
		latestValue := metric.latestSample(latestMetric)
		historyValues := extractMetric(metrics, metric.historySample)
		limit := computeControlLimit(metric.name, latestValue, historyValues)
		trend.ControlLimits = append(trend.ControlLimits, limit)
		if anomaly, ok := detectAnomaly(limit); ok {
			trend.Anomalies = append(trend.Anomalies, anomaly)
		}
		latestSeries := extractMetric(metrics, metric.latestSample)
		trend.PatternViolations = append(trend.PatternViolations, detectPatternViolations(metric.name, latestSeries, limit.Mean)...)
	}
	ewmaStates := []struct {
		metric string
		state  EWMAMetricState
	}{
		{metric: metricEWMASuccessRate, state: latestMetric.EWMASuccessRate},
		{metric: metricEWMACostUSD, state: latestMetric.EWMACostUSD},
		{metric: metricEWMADurationMs, state: latestMetric.EWMADurationMs},
	}
	for _, ewma := range ewmaStates {
		limit := TrendControlLimit{
			Metric: ewma.metric,
			Latest: ewma.state.Z,
			Mean:   ewma.state.Mean,
			StdDev: ewma.state.StdDev,
			UCL:    ewma.state.UCL,
			LCL:    ewma.state.LCL,
		}
		if anomaly, ok := detectAnomaly(limit); ok {
			trend.EWMAAnomalies = append(trend.EWMAAnomalies, anomaly)
		}
	}

	sort.Slice(trend.ControlLimits, func(i, j int) bool { return trend.ControlLimits[i].Metric < trend.ControlLimits[j].Metric })
	sort.Slice(trend.Anomalies, func(i, j int) bool { return trend.Anomalies[i].Metric < trend.Anomalies[j].Metric })
	sort.Slice(trend.EWMAAnomalies, func(i, j int) bool { return trend.EWMAAnomalies[i].Metric < trend.EWMAAnomalies[j].Metric })
	sort.Slice(trend.PatternViolations, func(i, j int) bool {
		if trend.PatternViolations[i].Metric == trend.PatternViolations[j].Metric {
			return trend.PatternViolations[i].Rule < trend.PatternViolations[j].Rule
		}
		return trend.PatternViolations[i].Metric < trend.PatternViolations[j].Metric
	})
	return trend
}

func computeEWMAState(metric string, value float64, values []float64, previousZ float64, hasPrevious bool) EWMAMetricState {
	z := value
	if hasPrevious {
		z = ewmaLambda*value + (1-ewmaLambda)*previousZ
	}
	mean, stddev := meanAndStdDev(values)
	scale := math.Sqrt(ewmaLambda / (2 - ewmaLambda))
	ucl := mean + ewmaSigmaMultiplier*stddev*scale
	lcl := mean - ewmaSigmaMultiplier*stddev*scale
	if isRateMetric(metric) {
		ucl = clamp(ucl, 0, 1)
		lcl = clamp(lcl, 0, 1)
	}
	return EWMAMetricState{
		Lambda: ewmaLambda,
		L:      ewmaSigmaMultiplier,
		Value:  value,
		Z:      z,
		Mean:   mean,
		StdDev: stddev,
		UCL:    ucl,
		LCL:    lcl,
	}
}

func computeProviderMetrics(entries []IterationMetric) []ProviderMetrics {
	if len(entries) == 0 {
		return []ProviderMetrics{}
	}

	type providerTotals struct {
		totalInvocations  int
		successes         int
		transportFailures int
		fallbacks         int
		totalDurationMs   int64
		totalCostUSD      float64
		totalInputTokens  int
		totalOutputTokens int
	}

	totalsByProvider := map[string]providerTotals{}
	for _, entry := range entries {
		name := resolveProviderName(entry.Provider, entry.Model)
		totals := totalsByProvider[name]
		totals.totalInvocations++
		if entry.Success {
			totals.successes++
		}
		if entry.FailureCategory == transportDisconnectFailure {
			totals.transportFailures++
		}
		if entry.Escalated {
			totals.fallbacks++
		}
		totals.totalDurationMs += entry.DurationMs
		totals.totalCostUSD += entry.CostUSD
		totals.totalInputTokens += entry.InputTokens
		totals.totalOutputTokens += entry.OutputTokens
		totalsByProvider[name] = totals
	}

	metrics := make([]ProviderMetrics, 0, len(totalsByProvider))
	for name, totals := range totalsByProvider {
		metrics = append(metrics, ProviderMetrics{
			Name:                 name,
			TotalInvocations:     totals.totalInvocations,
			Successes:            totals.successes,
			SuccessRate:          fraction(totals.successes, totals.totalInvocations),
			TransportFailures:    totals.transportFailures,
			TransportFailureRate: fraction(totals.transportFailures, totals.totalInvocations),
			FallbacksTriggered:   totals.fallbacks,
			AvgDurationMs:        averageInt64(totals.totalDurationMs, totals.totalInvocations),
			TotalCostUSD:         totals.totalCostUSD,
			TotalInputTokens:     totals.totalInputTokens,
			TotalOutputTokens:    totals.totalOutputTokens,
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Name < metrics[j].Name
	})
	return metrics
}

func fraction(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func averageInt64(total int64, count int) float64 {
	if count <= 0 {
		return 0
	}
	return float64(total) / float64(count)
}

func resolveProviderName(providerName, modelName string) string {
	if providerName != "" {
		return providerName
	}
	lowerModel := strings.ToLower(modelName)
	if strings.Contains(lowerModel, "gpt") || strings.Contains(lowerModel, "codex") {
		return "openai"
	}
	if lowerModel == "" {
		return "unknown"
	}
	return "claude"
}

func extractMetric(metrics []IterationMetric, pick func(IterationMetric) float64) []float64 {
	values := make([]float64, 0, len(metrics))
	for _, m := range metrics {
		values = append(values, pick(m))
	}
	return values
}

func boolToFloat64(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

func summarizePromptTokens(metrics []IterationMetric, windowSize int) PromptTokenSummary {
	summary := newPromptTokenSummary()
	if len(metrics) == 0 {
		return summary
	}

	start := len(metrics) - windowSize
	if start < 0 {
		start = 0
	}

	typeTotals := map[string]int{}
	typeCounts := map[string]int{}
	sectionTotals := map[string]int{}
	absDeltaPct := make([]float64, 0, len(metrics)-start)

	for i := start; i < len(metrics); i++ {
		diag := metrics[i].PromptDiagnostics
		if diag == nil {
			continue
		}

		typeTotals[diag.PromptType] += diag.EstimatedTokens
		typeCounts[diag.PromptType]++

		for section, tokens := range diag.SectionTokens {
			sectionTotals[section] += tokens
		}
		for _, action := range diag.ShapeActions {
			summary.BudgetActionFrequency[action]++
		}
		if diag.ReportedTokens > 0 {
			absDeltaPct = append(absDeltaPct, math.Abs(diag.TokenDeltaPct))
		}
	}

	for promptType, count := range typeCounts {
		if count <= 0 {
			continue
		}
		summary.ByPromptType = append(summary.ByPromptType, PromptTypeSummary{
			PromptType:         promptType,
			InvocationCount:    count,
			AvgEstimatedTokens: float64(typeTotals[promptType]) / float64(count),
		})
	}
	sort.Slice(summary.ByPromptType, func(i, j int) bool {
		if summary.ByPromptType[i].AvgEstimatedTokens == summary.ByPromptType[j].AvgEstimatedTokens {
			return summary.ByPromptType[i].PromptType < summary.ByPromptType[j].PromptType
		}
		return summary.ByPromptType[i].AvgEstimatedTokens > summary.ByPromptType[j].AvgEstimatedTokens
	})

	sections := make([]PromptSectionSummary, 0, len(sectionTotals))
	for section, tokens := range sectionTotals {
		sections = append(sections, PromptSectionSummary{
			Section:         section,
			EstimatedTokens: tokens,
		})
	}
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].EstimatedTokens == sections[j].EstimatedTokens {
			return sections[i].Section < sections[j].Section
		}
		return sections[i].EstimatedTokens > sections[j].EstimatedTokens
	})
	if len(sections) > promptSectionTopLimit {
		sections = sections[:promptSectionTopLimit]
	}
	summary.BySectionTop10 = sections

	summary.ReconciliationDrift = ReconciliationDrift{
		SampleCount:          len(absDeltaPct),
		MeanAbsTokenDeltaPct: meanFloat64(absDeltaPct),
		P95AbsTokenDeltaPct:  percentileFloat64(absDeltaPct, p95Percentile),
	}

	return summary
}

func computeControlLimit(metric string, latest float64, values []float64) TrendControlLimit {
	mean, stddev := meanAndStdDev(values)
	ucl := mean + controlLimitSigmaMultiplier*stddev
	lcl := mean - controlLimitSigmaMultiplier*stddev

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

	severity := anomalySeverityModerate
	if limit.StdDev == 0 {
		severity = anomalySeverityHigh
	} else {
		distance := math.Abs(limit.Latest-limit.Mean) / limit.StdDev
		if distance >= highSeveritySigmaThreshold {
			severity = anomalySeverityHigh
		}
	}

	dir := anomalyDirectionAbove
	if limit.Latest < limit.LCL {
		dir = anomalyDirectionBelow
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

func detectPatternViolations(metric string, values []float64, centerLine float64) []PatternViolation {
	if len(values) < nelsonRule2MinRunLength {
		return []PatternViolation{}
	}

	runAbove := trailingRunLength(values, func(v float64) bool { return v > centerLine })
	if runAbove >= nelsonRule2MinRunLength {
		return []PatternViolation{newRule2Violation(metric, anomalyDirectionAbove, runAbove, centerLine)}
	}

	runBelow := trailingRunLength(values, func(v float64) bool { return v < centerLine })
	if runBelow >= nelsonRule2MinRunLength {
		return []PatternViolation{newRule2Violation(metric, anomalyDirectionBelow, runBelow, centerLine)}
	}

	return []PatternViolation{}
}

func newRule2Violation(metric, direction string, runLength int, centerLine float64) PatternViolation {
	return PatternViolation{
		Metric:     metric,
		Rule:       nelsonRule2Name,
		Direction:  direction,
		RunLength:  runLength,
		CenterLine: centerLine,
		Message: fmt.Sprintf(
			"latest %d points are %s center line %.4f (Nelson Rule 2)",
			runLength,
			direction,
			centerLine,
		),
	}
}

func trailingRunLength(values []float64, match func(float64) bool) int {
	runLength := 0
	for i := len(values) - 1; i >= 0; i-- {
		if !match(values[i]) {
			break
		}
		runLength++
	}
	return runLength
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

	target := filepath.Join(metricsDir, iterationMetricsFilename)
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

	target := filepath.Join(metricsDir, processTrendFilename)
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
	var validationDurationTotal int64
	var validationDurationCount int
	var costTotal float64
	var mttrTotal int64
	var mttrCount int
	durations := make([]int64, 0, len(window))
	validationDurations := make([]int64, 0, len(window))
	beadCosts := map[string]beadCostAccum{}

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
		if !e.Success {
			switch e.FailurePhase {
			case failurephase.Preflight:
				preflightFailures++
			case failurephase.Build:
				buildFailures++
			case failurephase.Validation:
				validationFailures++
			case failurephase.Timeout:
				timeoutFailures++
			}
		}

		durationTotal += e.DurationMs
		if e.ValidationDurationMs > 0 {
			validationDurationTotal += e.ValidationDurationMs
			validationDurationCount++
			validationDurations = append(validationDurations, e.ValidationDurationMs)
		}
		costTotal += e.CostUSD
		durations = append(durations, e.DurationMs)

		updateBeadCostAccum(beadCosts, e)

		if e.MTTRProxyMs > 0 {
			mttrTotal += e.MTTRProxyMs
			mttrCount++
		}
	}

	count := float64(len(window))
	avgDuration := float64(durationTotal) / count
	avgValidationDuration := 0.0
	if validationDurationCount > 0 {
		avgValidationDuration = float64(validationDurationTotal) / float64(validationDurationCount)
	}
	avgCost := costTotal / count
	avgMTTR := 0.0
	if mttrCount > 0 {
		avgMTTR = float64(mttrTotal) / float64(mttrCount)
	}

	avgCostPerBead := averageCompletedBeadCost(beadCosts)

	return ProcessTrendWindow{
		SuccessRate:           float64(successes) / count,
		FailureRate:           float64(len(window)-successes) / count,
		FirstPassSuccess:      float64(firstPasses) / count,
		EscalationRate:        float64(escalations) / count,
		AvgDurationMs:         avgDuration,
		P95DurationMs:         percentileInt64(durations, p95Percentile),
		AvgValidationMs:       avgValidationDuration,
		P95ValidationMs:       percentileInt64(validationDurations, p95Percentile),
		AvgCostUSD:            avgCost,
		AvgCostPerBeadUSD:     avgCostPerBead,
		AvgMTTRProxyMs:        avgMTTR,
		PreflightFailureRate:  float64(preflightFailures) / count,
		BuildFailureRate:      float64(buildFailures) / count,
		ValidationFailureRate: float64(validationFailures) / count,
		TimeoutFailureRate:    float64(timeoutFailures) / count,
	}
}

type beadCostAccum struct {
	totalCost  float64
	hasSuccess bool
}

func updateBeadCostAccum(beadCosts map[string]beadCostAccum, entry IterationLog) {
	if entry.BeadID == "" {
		return
	}
	accum := beadCosts[entry.BeadID]
	accum.totalCost += entry.CostUSD
	if entry.Success {
		accum.hasSuccess = true
	}
	beadCosts[entry.BeadID] = accum
}

// averageCompletedBeadCost returns average total bead cost for beads completed in the window.
func averageCompletedBeadCost(beadCosts map[string]beadCostAccum) float64 {
	var completedBeadCostSum float64
	var completedBeadCount int
	for _, accum := range beadCosts {
		if accum.hasSuccess {
			completedBeadCostSum += accum.totalCost
			completedBeadCount++
		}
	}
	if completedBeadCount == 0 {
		return 0
	}
	return completedBeadCostSum / float64(completedBeadCount)
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
