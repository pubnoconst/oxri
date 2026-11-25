package settings

import (
	"fmt"
	"strconv"

	"github.com/oxri/oxri-go/internal/db"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// FieldVitals is the field performance card: the 75th percentile of each
// Core Web Vital over the last 28 days of readers' page views, mobile first,
// rated by Google's thresholds (PROFILING.md).
func FieldVitals(stats []db.VitalStat) g.Node {
	header := h.Div(h.Class("card-header"),
		h.H2(g.Text("Field Performance")),
		h.P(h.Class("card-desc"), g.Text("Core Web Vitals from readers' own browsers, last 28 days. Each figure is the 75th percentile of page views, as Google assesses a site. Good: LCP ≤ 2.5 s, INP ≤ 200 ms, CLS ≤ 0.1.")),
	)
	if len(stats) == 0 {
		return h.Section(h.Class("settings-card"), header,
			h.P(h.Class("vitals-empty"), g.Text("No reader page views recorded yet. Views by you while signed in, and anything on a local server, are not counted.")))
	}

	byDevice := map[string][]db.VitalStat{}
	var devices []string
	for _, s := range stats {
		if _, ok := byDevice[s.Device]; !ok {
			devices = append(devices, s.Device)
		}
		byDevice[s.Device] = append(byDevice[s.Device], s)
	}

	var groups []g.Node
	for _, d := range devices {
		var cells, culprits []g.Node
		for _, s := range byDevice[d] {
			cells = append(cells, h.Div(h.Class("vital vital-"+s.Rating),
				h.Span(h.Class("vital-name"), g.Text(s.Metric)),
				h.Span(h.Class("vital-value"), g.Text(formatVital(s.Metric, s.P75))),
				h.Span(h.Class("vital-meta"), g.Textf("%s · %d views · %.0f%% good", ratingLabel(s.Rating), s.Count, s.Good*100)),
			))
			for _, t := range s.Targets {
				culprits = append(culprits, h.Li(h.Code(g.Text(t.Target)), g.Textf(" %s, %d poor", s.Metric, t.Count)))
			}
		}
		group := []g.Node{h.H3(h.Class("vitals-device"), g.Text(deviceLabel(d))), h.Div(h.Class("vitals-grid"), g.Group(cells))}
		if len(culprits) > 0 {
			group = append(group, h.P(h.Class("vitals-culprits-label"), g.Text("Elements behind poor views")), h.Ul(h.Class("vitals-culprits"), g.Group(culprits)))
		}
		groups = append(groups, h.Div(h.Class("vitals-group"), g.Group(group)))
	}
	return h.Section(h.Class("settings-card"), header, g.Group(groups))
}

func formatVital(metric string, v float64) string {
	if metric == "CLS" {
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
	if v >= 1000 {
		return fmt.Sprintf("%.1f s", v/1000)
	}
	return fmt.Sprintf("%.0f ms", v)
}

func ratingLabel(r string) string {
	switch r {
	case "good":
		return "Good"
	case "needs-improvement":
		return "Needs improvement"
	default:
		return "Poor"
	}
}

func deviceLabel(d string) string {
	if d == "mobile" {
		return "Mobile"
	}
	return "Desktop"
}
