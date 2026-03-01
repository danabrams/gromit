package runner

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/danabrams/gromit/internal/logger"
    "github.com/danabrams/gromit/internal/tracker"
)

const (
    metadataLabelKey      = "spc_dedupe_label"
    metadataCauseClassKey = "spc_cause_class"
    metadataStratumKey    = "spc_stratum"
    metadataTypeKey       = "spc_issue_type"
)

// TrendControlLimit mirrors the logger definition to keep the runner package self-contained.
type TrendControlLimit = logger.TrendControlLimit

// CauseClass defines the classification assigned to an SPC signal.
type CauseClass string

const (
    CauseClassSpecial CauseClass = "special_cause"
    CauseClassCommon  CauseClass = "common_cause"
    CauseClassStable  CauseClass = "stable"
)

// SPCCauseRecord captures the classification output for one metric + stratum.
type SPCCauseRecord struct {
    Metric                 string
    Stratum                string
    Class                  CauseClass
    Latest                 float64
    Limit                  *logger.TrendControlLimit
    Drift                  float64
    PersistenceWindowCount int
    DetectedAt             time.Time
}

// Identity returns the deterministic identifier used for dedupe/cooldown bookkeeping.
func (r SPCCauseRecord) Identity() string {
    stratum := r.Stratum
    if stratum == "" {
        stratum = "global"
    }
    return fmt.Sprintf("%s|%s|%s", r.Metric, stratum, r.Class)
}

// SPCTriageResult summarizes the tracker issue produced for a classification record.
type SPCTriageResult struct {
    Record  SPCCauseRecord
    IssueID string
}

// SPCCooldownStore tracks the timestamp when an auto-triage issue was last created for an identity.
type SPCCooldownStore interface {
    Get(identity string) time.Time
    Set(identity string, when time.Time)
}

// SPCAutoTriageConfig controls persistence gate and cooldown behavior.
type SPCAutoTriageConfig struct {
    PersistenceGate int
    Cooldown        time.Duration
    Guidance        map[CauseClass]string
    IssueType       map[CauseClass]string
}

func defaultSPCAutoTriageConfig() SPCAutoTriageConfig {
    return SPCAutoTriageConfig{
        PersistenceGate: 2,
        Cooldown:        7 * 24 * time.Hour,
        Guidance: map[CauseClass]string{
            CauseClassSpecial: "Investigate the incident in the phase/provider where the limit was breached.",
            CauseClassCommon:  "Avoid tampering—focus on systemic improvements for the broader process.",
        },
        IssueType: map[CauseClass]string{
            CauseClassSpecial: "bug",
            CauseClassCommon:  "task",
        },
    }
}

// SPCAutoTriager runs the SPC auto-triage workflow.
type SPCAutoTriager struct {
    client tracker.Client
    store  SPCCooldownStore
    config SPCAutoTriageConfig
    now    func() time.Time
}

// SPCAutoTriageOption configures the auto-triage service.
type SPCAutoTriageOption func(*SPCAutoTriager)

// WithNowFunc overrides the clock used by the service.
func WithNowFunc(fn func() time.Time) SPCAutoTriageOption {
    return func(s *SPCAutoTriager) {
        if fn != nil {
            s.now = fn
        }
    }
}

// WithConfigOverride replaces the default configuration.
func WithConfigOverride(cfg SPCAutoTriageConfig) SPCAutoTriageOption {
    return func(s *SPCAutoTriager) {
        s.config = cfg
    }
}

// NewSPCAutoTriager creates a new instance wired with the provided tracker client and cooldown store.
func NewSPCAutoTriager(client tracker.Client, store SPCCooldownStore, opts ...SPCAutoTriageOption) *SPCAutoTriager {
    t := &SPCAutoTriager{
        client: client,
        store:  store,
        config: defaultSPCAutoTriageConfig(),
        now:    time.Now,
    }
    for _, opt := range opts {
        opt(t)
    }
    return t
}

// Process evaluates each classification record and emits tracker issues as needed.
func (s *SPCAutoTriager) Process(ctx context.Context, records []SPCCauseRecord) ([]SPCTriageResult, error) {
    var results []SPCTriageResult
    now := s.now()
    for _, rec := range records {
        if rec.Class == CauseClassStable {
            continue
        }
        if rec.PersistenceWindowCount < s.config.PersistenceGate {
            continue
        }
        identity := rec.Identity()
        label := dedupeLabelForIdentity(identity)
        if openExists(ctx, s.client, label) {
            continue
        }
        last := s.store.Get(identity)
        if !last.IsZero() && now.Sub(last) < s.config.Cooldown {
            continue
        }
        issueType, ok := s.config.IssueType[rec.Class]
        if !ok {
            continue
        }
        req := tracker.CreateRequest{
            Title:       fmt.Sprintf("SPC signal: %s (%s)", rec.Metric, displayStratum(rec.Stratum)),
            Status:      tracker.StatusOpen,
        Metadata: map[string]string{
            metadataLabelKey:      label,
            metadataCauseClassKey: string(rec.Class),
            metadataStratumKey:    displayStratum(rec.Stratum),
            metadataTypeKey:       issueType,
        },
            Description: buildIssueDescription(rec, s.config.Guidance[rec.Class]),
        }
        item, err := s.client.Create(ctx, req)
        if err != nil {
            return nil, fmt.Errorf("creating tracker issue: %w", err)
        }
        s.store.Set(identity, now)
        results = append(results, SPCTriageResult{Record: rec, IssueID: item.ID})
    }
    return results, nil
}

func dedupeLabelForIdentity(identity string) string {
    return fmt.Sprintf("spc-signal:%s", identity)
}

func displayStratum(stratum string) string {
    if stratum == "" {
        return "global"
    }
    return stratum
}

func buildIssueDescription(rec SPCCauseRecord, guidance string) string {
    lines := []string{
        fmt.Sprintf("Metric: %s", rec.Metric),
        fmt.Sprintf("Stratum: %s", displayStratum(rec.Stratum)),
        fmt.Sprintf("Classification: %s", rec.Class),
        fmt.Sprintf("Latest: %.2f", rec.Latest),
    }
    if rec.Limit != nil {
        lines = append(lines, fmt.Sprintf("Control limits: [%.2f, %.2f] (mean %.2f)", rec.Limit.LCL, rec.Limit.UCL, rec.Limit.Mean))
    }
    if rec.Drift != 0 {
        lines = append(lines, fmt.Sprintf("Drift vs center: %.2f", rec.Drift))
    }
    if !rec.DetectedAt.IsZero() {
        lines = append(lines, fmt.Sprintf("First detected: %s", rec.DetectedAt.Format(time.RFC3339)))
    }
    if guidance != "" {
        lines = append(lines, "Guidance:", guidance)
    }
    return strings.Join(lines, "\n")
}

func openExists(ctx context.Context, client tracker.Client, label string) bool {
    return false
}
