package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

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
	metricRollingReworkRate        = "rolling_rework_rate"
	metricRollingEscalationRate    = "rolling_escalation_rate"
	metricRollingQualityScore      = "rolling_quality_score"
	metricRollingAvgDurationMs     = "rolling_avg_duration_ms"
	metricRollingAvgValidationMs   = "rolling_avg_validation_ms"
	metricRollingAvgCostUSD        = "rolling_avg_cost_usd"
	metricRollingAvgInputTokens    = "rolling_avg_input_tokens"
	metricRollingPreflightFailure  = "rolling_preflight_failure_rate"
	metricRollingBuildFailure      = "rolling_build_failure_rate"
	metricRollingValidationFailure = "rolling_validation_failure_rate"
	metricRollingTimeoutFailure    = "rolling_timeout_failure_rate"
	metricRollingAvgCostPerBeadUSD = "rolling_avg_cost_per_bead_usd"
	metricEWMASuccessRate          = "ewma_success_rate"
	metricEWMACostUSD              = "ewma_cost_usd"
	metricEWMADurationMs           = "ewma_duration_ms"
	metricEWMAInputTokens          = "ewma_input_tokens"
	transportDisconnectFailure     = "transport_disconnect"
	stratumProviderPrefix          = "provider:"
	stratumModelPrefix             = "model:"
)

var phaseRateMetrics = []string{
	metricRollingPreflightFailure,
	metricRollingBuildFailure,
	metricRollingValidationFailure,
	metricRollingTimeoutFailure,
}

// IterationMetric stores a single iteration with rolling-window process metrics.
type IterationMetric struct {
	Timestamp                    time.Time                 `json:"timestamp"`
	Iteration                    int                       `json:"iteration"`
	BeadID                       string                    `json:"bead_id"`
	Model                        string                    `json:"model"`
	ReasoningEffort              string                    `json:"reasoning_effort,omitempty"`
	Provider                     string                    `json:"provider,omitempty"`
	FailurePhase                 string                    `json:"failure_phase,omitempty"`
	DefectOriginPhase            string                    `json:"defect_origin_phase,omitempty"`
	FailureCategory              string                    `json:"failure_category,omitempty"`
	FailureAttribution           string                    `json:"failure_attribution,omitempty"`
	Success                      bool                      `json:"success"`
	FirstPassSuccess             bool                      `json:"first_pass_success"`
	Escalated                    bool                      `json:"escalated"`
	QualityScore                 float64                   `json:"quality_score"`
	DurationMs                   int64                     `json:"duration_ms"`
	ValidationDurationMs         int64                     `json:"validation_duration_ms,omitempty"`
	CostUSD                      float64                   `json:"cost_usd"`
	InputTokens                  int                       `json:"input_tokens,omitempty"`
	OutputTokens                 int                       `json:"output_tokens,omitempty"`
	MTTRProxyMs                  int64                     `json:"mttr_proxy_ms,omitempty"`
	RollingSuccessRate           float64                   `json:"rolling_success_rate"`
	RollingFailureRate           float64                   `json:"rolling_failure_rate"`
	RollingFirstPassSuccess      float64                   `json:"rolling_first_pass_success_rate"`
	RollingReworkRate            float64                   `json:"rolling_rework_rate"`
	RollingEscalationRate        float64                   `json:"rolling_escalation_rate"`
	RollingQualityScore          float64                   `json:"rolling_quality_score"`
	RollingAvgDurationMs         float64                   `json:"rolling_avg_duration_ms"`
	RollingP95DurationMs         float64                   `json:"rolling_p95_duration_ms"`
	RollingAvgValidationMs       float64                   `json:"rolling_avg_validation_ms"`
	RollingP95ValidationMs       float64                   `json:"rolling_p95_validation_ms"`
	RollingAvgCostUSD            float64                   `json:"rolling_avg_cost_usd"`
	RollingAvgInputTokens        float64                   `json:"rolling_avg_input_tokens"`
	RollingAvgCostPerBeadUSD     float64                   `json:"rolling_avg_cost_per_bead_usd"`
	RollingAvgMTTRProxyMs        float64                   `json:"rolling_avg_mttr_proxy_ms"`
	RollingPreflightFailureRate  float64                   `json:"rolling_preflight_failure_rate"`
	RollingBuildFailureRate      float64                   `json:"rolling_build_failure_rate"`
	RollingValidationFailureRate float64                   `json:"rolling_validation_failure_rate"`
	RollingTimeoutFailureRate    float64                   `json:"rolling_timeout_failure_rate"`
	RollingTimeoutDecompositionAttempts int                 `json:"rolling_timeout_decomposition_attempts"`
	RollingTimeoutDecompositionSuccessRate float64          `json:"rolling_timeout_decomposition_success_rate"`
	EWMASuccessRate              EWMAMetricState           `json:"ewma_success_rate"`
	EWMACostUSD                  EWMAMetricState           `json:"ewma_cost_usd"`
	EWMADurationMs               EWMAMetricState           `json:"ewma_duration_ms"`
	EWMAInputTokens              EWMAMetricState           `json:"ewma_input_tokens"`
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
	ReworkRate            float64 `json:"rework_rate"`
	EscalationRate        float64 `json:"escalation_rate"`
	QualityScore          float64 `json:"quality_score"`
	AvgDurationMs         float64 `json:"avg_duration_ms"`
	P95DurationMs         float64 `json:"p95_duration_ms"`
	AvgValidationMs       float64 `json:"avg_validation_ms"`
	P95ValidationMs       float64 `json:"p95_validation_ms"`
	AvgCostUSD            float64 `json:"avg_cost_usd"`
	AvgInputTokens        float64 `json:"avg_input_tokens"`
	AvgCostPerBeadUSD     float64 `json:"avg_cost_per_bead_usd"`
	AvgMTTRProxyMs        float64 `json:"avg_mttr_proxy_ms"`
	PreflightFailureRate  float64 `json:"preflight_failure_rate"`
	BuildFailureRate      float64 `json:"build_failure_rate"`
	ValidationFailureRate float64 `json:"validation_failure_rate"`
	TimeoutFailureRate    float64 `json:"timeout_failure_rate"`
	TimeoutDecompositionAttempts int   `json:"timeout_decomposition_attempts"`
	TimeoutDecompositionSuccessRate float64 `json:"timeout_decomposition_success_rate"`
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
	GeneratedAt             time.Time                      `json:"generated_at"`
	TotalIterations         int                            `json:"total_iterations"`
	WindowSize              int                            `json:"window_size"`
	LatestWindow            ProcessTrendWindow             `json:"latest_window"`
	PromptTokenSummary      PromptTokenSummary             `json:"prompt_token_summary"`
	ProviderMetrics         []ProviderMetrics              `json:"provider_metrics"`
	ControlLimits           []TrendControlLimit            `json:"control_limits"`
	StratifiedControlLimits map[string][]TrendControlLimit `json:"stratified_control_limits"`
	Anomalies               []TrendAnomaly                 `json:"anomalies"`
	StratifiedAnomalies     map[string][]TrendAnomaly      `json:"stratified_anomalies"`
	EWMAAnomalies           []TrendAnomaly                 `json:"ewma_anomalies"`
	PatternViolations       []PatternViolation             `json:"pattern_violations"`
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
	if t.StratifiedControlLimits == nil {
		t.StratifiedControlLimits = map[string][]TrendControlLimit{}
	}
	if t.Anomalies == nil {
		t.Anomalies = []TrendAnomaly{}
	}
	if t.StratifiedAnomalies == nil {
		t.StratifiedAnomalies = map[string][]TrendAnomaly{}
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

	for key, limits := range t.StratifiedControlLimits {
		if limits == nil {
			t.StratifiedControlLimits[key] = []TrendControlLimit{}
		}
	}
	for key, anomalies := range t.StratifiedAnomalies {
		if anomalies == nil {
			t.StratifiedAnomalies[key] = []TrendAnomaly{}
		}
	}
}

func buildProcessTrend(metrics []IterationMetric, windowSize int) *ProcessTrend {
	trend := &ProcessTrend{
		GeneratedAt:             time.Now().UTC(),
		TotalIterations:         len(metrics),
		WindowSize:              windowSize,
		LatestWindow:            ProcessTrendWindow{},
		PromptTokenSummary:      newPromptTokenSummary(),
		ProviderMetrics:         []ProviderMetrics{},
		ControlLimits:           []TrendControlLimit{},
		StratifiedControlLimits: map[string][]TrendControlLimit{},
		Anomalies:               []TrendAnomaly{},
		StratifiedAnomalies:     map[string][]TrendAnomaly{},
		EWMAAnomalies:           []TrendAnomaly{},
		PatternViolations:       []PatternViolation{},
	}
	if len(metrics) == 0 {
		return trend
	}

	latestMetric := metrics[len(metrics)-1]
	trend.LatestWindow = ProcessTrendWindow{
		SuccessRate:           latestMetric.RollingSuccessRate,
		FailureRate:           latestMetric.RollingFailureRate,
		FirstPassSuccess:      latestMetric.RollingFirstPassSuccess,
		ReworkRate:            latestMetric.RollingReworkRate,
		EscalationRate:        latestMetric.RollingEscalationRate,
		QualityScore:          latestMetric.RollingQualityScore,
		AvgDurationMs:         latestMetric.RollingAvgDurationMs,
		P95DurationMs:         latestMetric.RollingP95DurationMs,
		AvgValidationMs:       latestMetric.RollingAvgValidationMs,
		P95ValidationMs:       latestMetric.RollingP95ValidationMs,
		AvgCostUSD:            latestMetric.RollingAvgCostUSD,
		AvgInputTokens:        latestMetric.RollingAvgInputTokens,
		AvgCostPerBeadUSD:     latestMetric.RollingAvgCostPerBeadUSD,
		AvgMTTRProxyMs:        latestMetric.RollingAvgMTTRProxyMs,
		PreflightFailureRate:  latestMetric.RollingPreflightFailureRate,
		BuildFailureRate:      latestMetric.RollingBuildFailureRate,
		ValidationFailureRate: latestMetric.RollingValidationFailureRate,
		TimeoutFailureRate:    latestMetric.RollingTimeoutFailureRate,
		TimeoutDecompositionAttempts: latestMetric.RollingTimeoutDecompositionAttempts,
		TimeoutDecompositionSuccessRate: latestMetric.RollingTimeoutDecompositionSuccessRate,
	}
	trend.PromptTokenSummary = summarizePromptTokens(metrics, windowSize)
	windowStart := len(metrics) - windowSize
	if windowStart < 0 {
		windowStart = 0
	}
	windowEntries := metrics[windowStart:]
	trend.ProviderMetrics = computeProviderMetrics(windowEntries)

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
	for _, ewma := range trendEWMASeries {
		limit := controlLimitFromEWMAState(ewma.name, ewma.state(latestMetric))
		if anomaly, ok := detectAnomaly(limit); ok {
			trend.EWMAAnomalies = append(trend.EWMAAnomalies, anomaly)
		}
	}
	stratifiedLimits, stratifiedAnomalies := buildStratifiedControlLimits(metrics)
	trend.StratifiedControlLimits = stratifiedLimits
	trend.StratifiedAnomalies = stratifiedAnomalies

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
