package storage

import (
	"strings"
	"testing"
	"time"
)

func TestBalanceTrendSpecForRange(t *testing.T) {
	now := time.Date(2026, 6, 18, 15, 42, 30, 0, time.FixedZone("CST", 8*60*60))
	for _, tc := range []struct {
		raw            string
		wantRange      string
		wantSince      time.Time
		wantBucketExpr string
	}{
		{
			raw:            "24h",
			wantRange:      "24h",
			wantSince:      time.Date(2026, 6, 17, 15, 42, 0, 0, now.Location()),
			wantBucketExpr: "3 minutes",
		},
		{
			raw:            "7d",
			wantRange:      "7d",
			wantSince:      time.Date(2026, 6, 12, 0, 0, 0, 0, now.Location()),
			wantBucketExpr: "hour",
		},
		{
			raw:            "30d",
			wantRange:      "30d",
			wantSince:      time.Date(2026, 5, 20, 0, 0, 0, 0, now.Location()),
			wantBucketExpr: "day",
		},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, ok := balanceTrendSpecForRange(tc.raw, now)
			if !ok {
				t.Fatalf("balanceTrendSpecForRange(%q) ok = false", tc.raw)
			}
			if got.Range != tc.wantRange {
				t.Fatalf("Range = %q, want %q", got.Range, tc.wantRange)
			}
			if !got.Since.Equal(tc.wantSince) {
				t.Fatalf("Since = %s, want %s", got.Since, tc.wantSince)
			}
			if !strings.Contains(got.BucketExpr, tc.wantBucketExpr) {
				t.Fatalf("BucketExpr = %q, want it to contain %q", got.BucketExpr, tc.wantBucketExpr)
			}
		})
	}
}

func TestBalanceTrendSpecForRangeRejectsUnknownRange(t *testing.T) {
	if _, ok := balanceTrendSpecForRange("90d", time.Now()); ok {
		t.Fatal("balanceTrendSpecForRange accepted unknown range")
	}
}

func TestStaleRateSnapshotModelNames(t *testing.T) {
	existing := []RateSnapshot{
		{ModelName: "default"},
		{ModelName: "vip"},
		{ModelName: "removed"},
		{ModelName: "removed"},
	}
	got := staleRateSnapshotModelNames(existing, []string{"default", "vip"})
	if strings.Join(got, ",") != "removed" {
		t.Fatalf("staleRateSnapshotModelNames() = %#v, want [removed]", got)
	}
}
