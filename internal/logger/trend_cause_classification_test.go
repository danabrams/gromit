package logger

import "testing"

func TestCauseClassificationRecordIdentity(t *testing.T) {
    testCases := []struct {
        name   string
        record CauseClassificationRecord
        want   string
    }{
        {
            name: "global",
            record: CauseClassificationRecord{
                Metric: "rolling_avg_cost_usd",
                Class:  CauseClassSpecial,
            },
            want: "rolling_avg_cost_usd|global|special_cause",
        },
        {
            name: "provider stratum",
            record: CauseClassificationRecord{
                Metric:  "rolling_avg_duration_ms",
                Stratum: "provider:claude",
                Class:   CauseClassCommon,
            },
            want: "rolling_avg_duration_ms|provider:claude|common_cause",
        },
    }

    for _, tc := range testCases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
            if got := tc.record.Identity(); got != tc.want {
                t.Fatalf("got identity %q, want %q", got, tc.want)
            }
        })
    }
}
