package logger

import "time"

// CauseClass enumerates the SPC cause classification assigned to a metric.
type CauseClass string

const (
	CauseClassSpecial CauseClass = "special_cause"
	CauseClassCommon  CauseClass = "common_cause"
	CauseClassStable  CauseClass = "stable"
)

// CauseClassificationRecord captures the trend classification output for a single metric and stratum.
type CauseClassificationRecord struct {
	Metric             string             `json:"metric"`
	Stratum            string             `json:"stratum,omitempty"`
	Class              CauseClass         `json:"class"`
	Latest             float64            `json:"latest"`
	Drift              float64            `json:"drift,omitempty"`
	Limit              *TrendControlLimit `json:"limit,omitempty"`
	PersistenceWindows int                `json:"persistence_windows"`
	DetectedAt         time.Time          `json:"detected_at,omitempty"`
	Severity           string             `json:"severity,omitempty"`
}
