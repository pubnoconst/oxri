package settings

import (
	"strings"
	"testing"

	"github.com/oxri/oxri-go/internal/db"
)

func TestFieldVitalsShowsP75RatedAndCulprits(t *testing.T) {
	var b strings.Builder
	_ = FieldVitals([]db.VitalStat{
		{Metric: "LCP", Device: "mobile", Count: 40, P75: 3100, Rating: "needs-improvement", Good: 0.6,
			Targets: []db.TargetCount{{Target: "p.journal-entry-excerpt", Count: 4}}},
		{Metric: "CLS", Device: "mobile", Count: 40, P75: 0.02, Rating: "good", Good: 0.95},
		{Metric: "INP", Device: "desktop", Count: 9, P75: 80, Rating: "good", Good: 1},
	}).Render(&b)
	got := b.String()
	for _, want := range []string{
		"Field Performance", "Mobile", "Desktop",
		`vital vital-needs-improvement`, "3.1 s", "Needs improvement · 40 views · 60% good",
		"0.02", "80 ms", "p.journal-entry-excerpt", "LCP, 4 poor",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("card lacks %q", want)
		}
	}
	if strings.Index(got, "Mobile") > strings.Index(got, "Desktop") {
		t.Error("mobile, the reference reader, comes first")
	}

	b.Reset()
	_ = FieldVitals(nil).Render(&b)
	if !strings.Contains(b.String(), "No reader page views recorded yet") {
		t.Error("empty state")
	}
}
