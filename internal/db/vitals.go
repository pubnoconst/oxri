package db

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync/atomic"
	"time"
)

// Field performance: Core Web Vitals reported by readers' own browsers
// (assets/js/vitals.js, routes/vitals.go). One row per metric per page view;
// nothing that identifies a reader is kept (no address, no agent string, no
// id): only the page, the metric, and a coarse class of device and network.

const vitalsTableDDL = `
CREATE TABLE IF NOT EXISTS vitals (
  at INTEGER NOT NULL,
  page TEXT NOT NULL CHECK(length(page) BETWEEN 1 AND 32),
  metric TEXT NOT NULL CHECK(metric IN ('LCP', 'INP', 'CLS', 'TTFB')),
  value REAL NOT NULL CHECK(value >= 0 AND value <= 120000),
  rating TEXT NOT NULL CHECK(rating IN ('good', 'needs-improvement', 'poor')),
  device TEXT NOT NULL CHECK(device IN ('mobile', 'desktop')),
  network TEXT NOT NULL CHECK(length(network) <= 8),
  memory REAL NOT NULL CHECK(memory >= 0 AND memory <= 64),
  target TEXT NOT NULL CHECK(length(target) <= 100)
);`

const vitalsIndexDDL = `CREATE INDEX IF NOT EXISTS vitals_at ON vitals (at);`

// VitalsWindow is how far back the field summary looks (as Chrome's own
// field data does: 28 days).
const VitalsWindow = 28 * 24 * time.Hour

// vitalsMaxRows bounds the table however much traffic arrives.
const vitalsMaxRows = 20000

// ErrInvalidVital is a sample outside the table's rules.
var ErrInvalidVital = errors.New("invalid vital")

// Vital is one metric from one page view.
type Vital struct {
	At      time.Time
	Page    string // a route template, e.g. "/journals/:id"
	Metric  string // LCP, INP, CLS, TTFB
	Value   float64
	Rating  string
	Device  string // mobile or desktop
	Network string // effective connection type, e.g. "4g"; "" if unknown
	Memory  float64
	Target  string // the element the browser attributed it to (a CSS selector)
}

var vitalsInserts atomic.Int64

// PutVitals stores one page view's samples. Every 200th call also prunes
// rows older than VitalsWindow and beyond vitalsMaxRows.
func (s *Store) PutVitals(ctx context.Context, vs []Vital) error {
	if len(vs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, v := range vs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO vitals (at, page, metric, value, rating, device, network, memory, target) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`,
			v.At.UnixMilli(), v.Page, v.Metric, v.Value, v.Rating, v.Device, v.Network, v.Memory, v.Target,
		); err != nil {
			return errors.Join(ErrInvalidVital, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if vitalsInserts.Add(1)%200 == 0 {
		_ = s.PruneVitals(ctx, time.Now())
	}
	return nil
}

// PruneVitals drops samples older than VitalsWindow, then the oldest beyond
// vitalsMaxRows.
func (s *Store) PruneVitals(ctx context.Context, now time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM vitals WHERE at < ?;`, now.Add(-VitalsWindow).UnixMilli()); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM vitals WHERE rowid IN (SELECT rowid FROM vitals ORDER BY at DESC LIMIT -1 OFFSET ?);`, vitalsMaxRows)
	return err
}

// VitalStat is one metric's field result for one device class: Google
// assesses a page by the 75th percentile of its page views.
type VitalStat struct {
	Metric  string
	Device  string
	Count   int
	P75     float64
	Rating  string  // of the p75, by Google's thresholds
	Good    float64 // share of views rated good, 0..1
	Poor    float64 // share rated poor
	Targets []TargetCount
}

// TargetCount is an element that caused poor samples, and how many.
type TargetCount struct {
	Target string
	Count  int
}

// Thresholds are Google's Core Web Vitals boundaries: at or under Good is
// good, over Poor is poor (LCP and TTFB in ms, INP in ms, CLS unitless).
var Thresholds = map[string][2]float64{
	"LCP":  {2500, 4000},
	"INP":  {200, 500},
	"CLS":  {0.1, 0.25},
	"TTFB": {800, 1800},
}

// RateVital rates a value by Thresholds.
func RateVital(metric string, v float64) string {
	t, ok := Thresholds[metric]
	switch {
	case !ok:
		return ""
	case v <= t[0]:
		return "good"
	case v <= t[1]:
		return "needs-improvement"
	default:
		return "poor"
	}
}

// VitalsSummary is the p75 of each metric by device class over VitalsWindow,
// optionally for one page ("" for all), with the elements behind poor views.
func (s *Store) VitalsSummary(ctx context.Context, now time.Time, page string) ([]VitalStat, error) {
	q := `SELECT metric, device, value, rating, target FROM vitals WHERE at >= ?`
	args := []any{now.Add(-VitalsWindow).UnixMilli()}
	if page != "" {
		q += ` AND page = ?`
		args = append(args, page)
	}
	rows, err := s.db.QueryContext(ctx, q+`;`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type key struct{ metric, device string }
	type acc struct {
		values      []float64
		good, poor  int
		poorTargets map[string]int
	}
	groups := map[key]*acc{}
	for rows.Next() {
		var metric, device, rating, target string
		var value float64
		if err := rows.Scan(&metric, &device, &value, &rating, &target); err != nil {
			return nil, err
		}
		k := key{metric, device}
		a := groups[k]
		if a == nil {
			a = &acc{poorTargets: map[string]int{}}
			groups[k] = a
		}
		a.values = append(a.values, value)
		switch rating {
		case "good":
			a.good++
		case "poor":
			a.poor++
			if target != "" {
				a.poorTargets[target]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []VitalStat
	for k, a := range groups {
		p75 := Percentile(a.values, 75)
		st := VitalStat{
			Metric: k.metric, Device: k.device, Count: len(a.values), P75: p75,
			Rating: RateVital(k.metric, p75),
			Good:   float64(a.good) / float64(len(a.values)),
			Poor:   float64(a.poor) / float64(len(a.values)),
		}
		for t, n := range a.poorTargets {
			st.Targets = append(st.Targets, TargetCount{t, n})
		}
		sort.Slice(st.Targets, func(i, j int) bool {
			if st.Targets[i].Count != st.Targets[j].Count {
				return st.Targets[i].Count > st.Targets[j].Count
			}
			return st.Targets[i].Target < st.Targets[j].Target
		})
		if len(st.Targets) > 3 {
			st.Targets = st.Targets[:3]
		}
		out = append(out, st)
	}
	order := map[string]int{"LCP": 0, "INP": 1, "CLS": 2, "TTFB": 3}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Device != out[j].Device {
			return out[i].Device == "mobile" // the reference reader first
		}
		return order[out[i].Metric] < order[out[j].Metric]
	})
	return out, nil
}

// VitalPages lists the pages with samples in the window, most viewed first.
func (s *Store) VitalPages(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT page FROM vitals WHERE at >= ? GROUP BY page ORDER BY count(*) DESC, page;`, now.Add(-VitalsWindow).UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pages []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

// Percentile is the nearest-rank p-th percentile of vs (0 for none).
func Percentile(vs []float64, p float64) float64 {
	if len(vs) == 0 {
		return 0
	}
	s := append([]float64(nil), vs...)
	sort.Float64s(s)
	rank := int(math.Ceil(p / 100 * float64(len(s))))
	if rank < 1 {
		rank = 1
	}
	return s[rank-1]
}
