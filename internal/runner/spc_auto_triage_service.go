package runner

import (
	"context"
	"fmt"
	"github.com/danabrams/gromit/internal/analytics"
	"time"

	"github.com/danabrams/gromit/internal/state"
	"github.com/danabrams/gromit/internal/tracker"
)

// SPCAutoTriageService evaluates SPC signals after a run and wires automated triage logic.
type SPCAutoTriageService interface {
	// EvaluateAndTriage runs the auto-triage workflow and returns any diagnostic error encountered.
	EvaluateAndTriage(ctx context.Context) error
}

type noopSPCAutoTriageService struct{}

func newSPCAutoTriageService(paths []string, client tracker.Client, store SPCCooldownStore) SPCAutoTriageService {
	normalized := normalizeTrendPaths(paths)
	if client == nil || len(normalized) == 0 {
		return &noopSPCAutoTriageService{}
	}
	if store == nil {
		store = noopSPCCooldownStore{}
	}
	return &spcAutoTriageService{
		triager:    NewSPCAutoTriager(client, store),
		trendPaths: normalized,
	}
}

func (noopSPCAutoTriageService) EvaluateAndTriage(ctx context.Context) error {
	return nil
}

type spcAutoTriageService struct {
	triager    *SPCAutoTriager
	trendPaths []string
}

func (s *spcAutoTriageService) EvaluateAndTriage(ctx context.Context) error {
	if s == nil || s.triager == nil || len(s.trendPaths) == 0 {
		return nil
	}
	records, err := loadSPCCauseRecords(s.trendPaths)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	_, err = s.triager.Process(ctx, records)
	return err
}

func loadSPCCauseRecords(paths []string) ([]SPCCauseRecord, error) {
	var lastErr error
	for _, path := range paths {
		if path == "" {
			continue
		}
		trend, err := analytics.ReadProcessTrend(path)
		if err != nil {
			lastErr = fmt.Errorf("reading process trend (%s): %w", path, err)
			continue
		}
		if trend == nil {
			continue
		}
		return convertCauseRecords(trend.CauseClassifications), nil
	}
	return nil, lastErr
}

func convertCauseRecords(records []analytics.CauseClassificationRecord) []SPCCauseRecord {
	out := make([]SPCCauseRecord, 0, len(records))
	for _, rec := range records {
		out = append(out, SPCCauseRecord{
			Metric:                 rec.Metric,
			Stratum:                rec.Stratum,
			Class:                  CauseClass(rec.Class),
			Latest:                 rec.Latest,
			Limit:                  rec.Limit,
			Drift:                  rec.Drift,
			PersistenceWindowCount: rec.PersistenceWindows,
			DetectedAt:             rec.DetectedAt,
		})
	}
	return out
}

func normalizeTrendPaths(paths []string) []string {
	seen := make(map[string]struct{})
	var normalized []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		normalized = append(normalized, path)
	}
	return normalized
}

func newStateCooldownStore(sf *state.File) SPCCooldownStore {
	return &stateCooldownStore{sf: sf}
}

type stateCooldownStore struct {
	sf *state.File
}

func (s *stateCooldownStore) Get(identity string) time.Time {
	if s == nil || s.sf == nil {
		return time.Time{}
	}
	return s.sf.GetSPCCooldown(identity)
}

func (s *stateCooldownStore) Set(identity string, when time.Time) {
	if s == nil || s.sf == nil {
		return
	}
	s.sf.SetSPCCooldown(identity, when)
}

type noopSPCCooldownStore struct{}

func (noopSPCCooldownStore) Get(identity string) time.Time {
	return time.Time{}
}

func (noopSPCCooldownStore) Set(identity string, when time.Time) {}
