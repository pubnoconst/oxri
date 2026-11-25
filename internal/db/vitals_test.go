package db

import (
	"context"
	"testing"
	"time"
)

func TestPercentileIsNearestRank(t *testing.T) {
	vs := []float64{5, 1, 4, 2, 3, 6, 7, 8}
	for p, want := range map[float64]float64{75: 6, 50: 4, 100: 8, 1: 1} {
		if got := Percentile(vs, p); got != want {
			t.Errorf("p%.0f = %v, want %v", p, got, want)
		}
	}
	if Percentile(nil, 75) != 0 {
		t.Error("empty")
	}
}

func TestVitalsSummaryAndPruning(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, ":memory:", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	now := time.Now()

	var vs []Vital
	for i, v := range []float64{1000, 1500, 2000, 2400, 5000} {
		vs = append(vs, Vital{At: now.Add(-time.Duration(i) * time.Hour), Page: "/journals", Metric: "LCP", Value: v,
			Rating: RateVital("LCP", v), Device: "mobile", Target: "p.excerpt"})
	}
	vs = append(vs, Vital{At: now.Add(-40 * 24 * time.Hour), Page: "/journals", Metric: "LCP", Value: 9000, Rating: "poor", Device: "mobile"})
	if err := store.PutVitals(ctx, vs); err != nil {
		t.Fatal(err)
	}

	stats, err := store.VitalsSummary(ctx, now, "")
	if err != nil || len(stats) != 1 {
		t.Fatalf("summary: %+v %v", stats, err)
	}
	s := stats[0]
	if s.Count != 5 || s.P75 != 2400 || s.Rating != "good" {
		t.Errorf("p75 over the 28-day window: %+v (the 40-day-old sample must not count)", s)
	}
	if len(s.Targets) != 1 || s.Targets[0].Target != "p.excerpt" || s.Targets[0].Count != 1 {
		t.Errorf("poor targets: %+v", s.Targets)
	}

	if err := store.PruneVitals(ctx, now); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = store.db.QueryRowContext(ctx, `SELECT count(*) FROM vitals;`).Scan(&n)
	if n != 5 {
		t.Errorf("after pruning: %d rows, want 5", n)
	}

	bad := []Vital{{At: now, Page: "/", Metric: "FCP", Value: 1, Rating: "good", Device: "mobile"}}
	if err := store.PutVitals(ctx, bad); err == nil {
		t.Error("the table accepted a metric it does not keep")
	}
}

func TestRateVitalUsesGoogleThresholds(t *testing.T) {
	for _, c := range []struct {
		m    string
		v    float64
		want string
	}{
		{"LCP", 2500, "good"}, {"LCP", 2501, "needs-improvement"}, {"LCP", 4001, "poor"},
		{"INP", 200, "good"}, {"INP", 201, "needs-improvement"}, {"INP", 501, "poor"},
		{"CLS", 0.1, "good"}, {"CLS", 0.11, "needs-improvement"}, {"CLS", 0.26, "poor"},
	} {
		if got := RateVital(c.m, c.v); got != c.want {
			t.Errorf("%s %v = %s, want %s", c.m, c.v, got, c.want)
		}
	}
}
