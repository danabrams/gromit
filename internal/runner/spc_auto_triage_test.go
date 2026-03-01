package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danabrams/gromit/internal/logger"
	"github.com/danabrams/gromit/internal/tracker"
	"github.com/danabrams/gromit/internal/tracker/trackertest"
)

func TestSPCAutoTriage_CreatesIssuesForClassificationPermutations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var createdRequests []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		createdRequests = append(createdRequests, req)
		return &tracker.Item{ID: fmt.Sprintf("issue-%d", len(createdRequests)), Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	cooldownStore := newTestCooldownStore()
	triager := NewSPCAutoTriager(trackerClient, cooldownStore, WithNowFunc(func() time.Time { return now }))

	const (
		metricCost        = "rolling_avg_cost_usd"
		metricDuration    = "rolling_avg_duration_ms"
		metricInputTokens = "rolling_avg_input_tokens"
	)

	records := []SPCCauseRecord{
		{
			Metric:                 metricCost,
			Stratum:                "",
			Class:                  CauseClassSpecial,
			PersistenceWindowCount: 2,
			Latest:                 123.4,
			DetectedAt:             now.Add(-2 * time.Hour),
			Limit: &TrendControlLimit{
				Metric: metricCost,
				Latest: 123.4,
				LCL:    100,
				UCL:    200,
			},
		},
		{
			Metric:                 metricDuration,
			Stratum:                "provider:claude",
			Class:                  CauseClassCommon,
			PersistenceWindowCount: 2,
			Latest:                 456,
			Drift:                  5,
			DetectedAt:             now.Add(-30 * time.Minute),
		},
		{
			Metric:                 metricInputTokens,
			Stratum:                "model:ultra",
			Class:                  CauseClassStable,
			PersistenceWindowCount: 4,
			Latest:                 789,
			DetectedAt:             now.Add(-15 * time.Minute),
		},
	}

	results, err := triager.Process(ctx, records)
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 triage outcomes, got %d", len(results))
	}
	if len(createdRequests) != 2 {
		t.Fatalf("expected 2 tracker.Create calls, got %d", len(createdRequests))
	}

	for _, req := range createdRequests {
		dedupeLabel, ok := req.Metadata[metadataLabelKey]
		if !ok || !strings.HasPrefix(dedupeLabel, "spc-signal:") {
			t.Errorf("dedupe label missing or malformed in request metadata: %v", req.Metadata)
		}
	}

	var seenSpecial, seenCommon bool
	for _, req := range createdRequests {
		switch req.Metadata[metadataCauseClassKey] {
		case string(CauseClassSpecial):
			seenSpecial = true
		case string(CauseClassCommon):
			seenCommon = true
		default:
			t.Errorf("unexpected cause class %q", req.Metadata[metadataCauseClassKey])
		}
	}
	if !seenSpecial || !seenCommon {
		t.Fatalf("expected both special and common triage actions, got special=%v common=%v", seenSpecial, seenCommon)
	}
}

func TestSPCAutoTriage_IncludesGuidanceForEachCauseClass(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: fmt.Sprintf("guidance-%d", len(created)), Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	store := newTestCooldownStore()
	cfg := SPCAutoTriageConfig{
		PersistenceGate: 2,
		Cooldown:        7 * 24 * time.Hour,
		Guidance: map[CauseClass]string{
			CauseClassSpecial: "special guidance is included",
			CauseClassCommon:  "common guidance is included",
		},
		IssueType: map[CauseClass]string{
			CauseClassSpecial: "bug",
			CauseClassCommon:  "task",
		},
	}
	triager := NewSPCAutoTriager(trackerClient, store, WithNowFunc(func() time.Time { return now }), WithConfigOverride(cfg))

	records := []SPCCauseRecord{
		{Metric: "c1", Stratum: "global", Class: CauseClassSpecial, PersistenceWindowCount: 2, Latest: 10, DetectedAt: now.Add(-time.Hour)},
		{Metric: "c2", Stratum: "provider:foo", Class: CauseClassCommon, PersistenceWindowCount: 2, Latest: 20, DetectedAt: now.Add(-time.Hour)},
	}

	_, err := triager.Process(ctx, records)
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}

	if len(created) != 2 {
		t.Fatalf("expected 2 create requests, got %d", len(created))
	}
	for _, req := range created {
		switch req.Metadata[metadataCauseClassKey] {
		case string(CauseClassSpecial):
			if !strings.Contains(req.Description, cfg.Guidance[CauseClassSpecial]) {
				t.Errorf("special issue description missing guidance: %s", req.Description)
			}
		case string(CauseClassCommon):
			if !strings.Contains(req.Description, cfg.Guidance[CauseClassCommon]) {
				t.Errorf("common issue description missing guidance: %s", req.Description)
			}
		default:
			t.Errorf("unexpected cause class %q", req.Metadata[metadataCauseClassKey])
		}
	}
}

func TestSPCAutoTriage_SetsIssueTypeMapping(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: fmt.Sprintf("type-%d", len(created)), Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	store := newTestCooldownStore()
	triager := NewSPCAutoTriager(trackerClient, store, WithNowFunc(func() time.Time { return now }))

	records := []SPCCauseRecord{
		{Metric: "c1", Stratum: "global", Class: CauseClassSpecial, PersistenceWindowCount: 2, Latest: 10, DetectedAt: now.Add(-time.Hour)},
		{Metric: "c2", Stratum: "provider:foo", Class: CauseClassCommon, PersistenceWindowCount: 2, Latest: 20, DetectedAt: now.Add(-time.Hour)},
	}

	_, err := triager.Process(ctx, records)
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if len(created) != 2 {
		t.Fatalf("expected 2 create requests, got %d", len(created))
	}
	for _, req := range created {
		switch req.Metadata[metadataCauseClassKey] {
		case string(CauseClassSpecial):
			if req.Metadata[metadataTypeKey] != defaultIssueType(CauseClassSpecial) {
				t.Errorf("special issue type = %q, want %q", req.Metadata[metadataTypeKey], defaultIssueType(CauseClassSpecial))
			}
		case string(CauseClassCommon):
			if req.Metadata[metadataTypeKey] != defaultIssueType(CauseClassCommon) {
				t.Errorf("common issue type = %q, want %q", req.Metadata[metadataTypeKey], defaultIssueType(CauseClassCommon))
			}
		default:
			t.Errorf("unexpected cause class %q", req.Metadata[metadataCauseClassKey])
		}
	}
}

func TestSPCAutoTriage_RequiresPersistenceGate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: fmt.Sprintf("pg-%d", len(created)), Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	store := newTestCooldownStore()
	triager := NewSPCAutoTriager(trackerClient, store, WithNowFunc(func() time.Time { return now }))

	records := []SPCCauseRecord{
		{Metric: "c1", Stratum: "global", Class: CauseClassSpecial, PersistenceWindowCount: 1, Latest: 10, DetectedAt: now.Add(-time.Hour)},
		{Metric: "c2", Stratum: "provider:foo", Class: CauseClassSpecial, PersistenceWindowCount: 2, Latest: 20, DetectedAt: now.Add(-time.Hour)},
	}

	_, err := triager.Process(ctx, records)
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected 1 create request, got %d", len(created))
	}
}

func TestSPCAutoTriage_RespectsDedupeLabels(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: fmt.Sprintf("dedupe-%d", len(created)), Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return []tracker.Item{{ID: "existing", Status: tracker.StatusOpen}}, nil
	}

	store := newTestCooldownStore()
	triager := NewSPCAutoTriager(trackerClient, store, WithNowFunc(func() time.Time { return now }))

	records := []SPCCauseRecord{
		{Metric: "c1", Stratum: "global", Class: CauseClassSpecial, PersistenceWindowCount: 2, Latest: 10, DetectedAt: now.Add(-time.Hour)},
	}

	_, err := triager.Process(ctx, records)
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if len(created) != 0 {
		t.Fatalf("expected dedupe to skip creation, got %d requests", len(created))
	}
}

func TestSPCAutoTriage_EnforcesCooldownBoundary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: fmt.Sprintf("cooldown-%d", len(created)), Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	store := newTestCooldownStore()
	store.Set("c1|global|special_cause", now.Add(-6*24*time.Hour))
	store.Set("c2|provider:foo|common_cause", now.Add(-7*24*time.Hour))

	triager := NewSPCAutoTriager(trackerClient, store, WithNowFunc(func() time.Time { return now }))

	records := []SPCCauseRecord{
		{Metric: "c1", Stratum: "global", Class: CauseClassSpecial, PersistenceWindowCount: 2, Latest: 10, DetectedAt: now.Add(-time.Hour)},
		{Metric: "c2", Stratum: "provider:foo", Class: CauseClassCommon, PersistenceWindowCount: 2, Latest: 20, DetectedAt: now.Add(-time.Hour)},
	}

	_, err := triager.Process(ctx, records)
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected cooldown to allow only one creation, got %d", len(created))
	}
	if created[0].Metadata[metadataCauseClassKey] != string(CauseClassCommon) {
		t.Fatalf("expected common cause to fire after cooldown, got %s", created[0].Metadata[metadataCauseClassKey])
	}
}

func TestSPCAutoTriage_IncludesEvidencePayload(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	trackerClient := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	trackerClient.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: "evidence-1", Status: tracker.StatusOpen}, nil
	}
	trackerClient.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	store := newTestCooldownStore()
	triager := NewSPCAutoTriager(trackerClient, store, WithNowFunc(func() time.Time { return now }))

	rec := SPCCauseRecord{
		Metric:                 "metric",
		Stratum:                "global",
		Class:                  CauseClassSpecial,
		PersistenceWindowCount: 2,
		Latest:                 42,
		Drift:                  3.14,
		DetectedAt:             now.Add(-time.Hour),
		Limit: &TrendControlLimit{
			Metric: "metric",
			Latest: 42,
			Mean:   40,
			LCL:    30,
			UCL:    50,
		},
	}

	_, err := triager.Process(ctx, []SPCCauseRecord{rec})
	if err != nil {
		t.Fatalf("Process() returned error: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected one creation, got %d", len(created))
	}
	desc := created[0].Description
	want := []string{"Metric: metric", "Classification: special_cause", "Latest: 42.00", "Control limits:", "Drift vs center: 3.14", "First detected:", "Guidance:"}
	for _, substring := range want {
		if !strings.Contains(desc, substring) {
			t.Errorf("description missing %q: %s", substring, desc)
		}
	}
}

func TestSPCAutoTriageService_EvaluatesProcessTrend(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)

	tempRoot := t.TempDir()
	metricsDir := filepath.Join(tempRoot, "metrics")
	if err := os.MkdirAll(metricsDir, 0o755); err != nil {
		t.Fatalf("failed to create metrics dir: %v", err)
	}
	trendPath := filepath.Join(metricsDir, "process_trend.json")
	trend := logger.ProcessTrend{
		GeneratedAt: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		CauseClassifications: []logger.CauseClassificationRecord{
			{
				Metric:             "rolling_avg_cost_usd",
				Stratum:            "",
				Class:              logger.CauseClassSpecial,
				Latest:             123.45,
				PersistenceWindows: 2,
				DetectedAt:         now.Add(-2 * time.Hour),
			},
		},
	}
	data, err := json.Marshal(trend)
	if err != nil {
		t.Fatalf("failed to marshal process trend: %v", err)
	}
	if err := os.WriteFile(trendPath, data, 0o644); err != nil {
		t.Fatalf("failed to write process trend: %v", err)
	}

	client := trackertest.NewStubTrackerClient()
	var created []tracker.CreateRequest
	client.CreateFn = func(ctx context.Context, req tracker.CreateRequest) (*tracker.Item, error) {
		created = append(created, req)
		return &tracker.Item{ID: fmt.Sprintf("triage-%d", len(created)), Status: tracker.StatusOpen}, nil
	}
	client.ListWithLabelFn = func(ctx context.Context, label string) ([]tracker.Item, error) {
		return nil, nil
	}

	store := newTestCooldownStore()
	service := newSPCAutoTriageService([]string{trendPath}, client, store)
	if service == nil {
		t.Fatal("expected service to be constructed")
	}

	if err := service.EvaluateAndTriage(ctx); err != nil {
		t.Fatalf("EvaluateAndTriage() error = %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("expected one tracker creation, got %d", len(created))
	}
}

type testCooldownStore struct {
	values map[string]time.Time
}

func newTestCooldownStore() *testCooldownStore {
	return &testCooldownStore{values: map[string]time.Time{}}
}

func (s *testCooldownStore) Get(identity string) time.Time {
	if s == nil {
		return time.Time{}
	}
	return s.values[identity]
}

func (s *testCooldownStore) Set(identity string, when time.Time) {
	if s == nil {
		return
	}
	s.values[identity] = when
}

func defaultIssueType(class CauseClass) string {
	return defaultSPCAutoTriageConfig().IssueType[class]
}
