package journals

import (
	"fmt"

	"github.com/oxri/oxri-go/internal/db"
)

// VacateSpaceView holds all state required to render /journals/vacate-space/.
type VacateSpaceView struct {
	Entries           []RenderedEntry
	RequiredBytes     int64
	PendingEntryBytes int64
	UsedBytes         int64
	LimitBytes        int64
	AutoPlan          db.AutoVacatePlan
	PendingSub        *db.PendingSubmission
	AutoResubmitReady bool
}

// HasPendingSubmission returns true when a stashed journal form is ready to be resubmitted.
func (v VacateSpaceView) HasPendingSubmission() bool {
	return v.PendingSub != nil
}

// ResubmitActionURL returns the form action URL (/journals or /journals/{id}) for resubmitting the pending entry.
func (v VacateSpaceView) ResubmitActionURL() string {
	if v.PendingSub != nil && v.PendingSub.IsEdit && v.PendingSub.EntryID > 0 {
		return fmt.Sprintf("/journals/%d", v.PendingSub.EntryID)
	}
	return "/journals"
}

// InitialProgressPercent returns 100 when RequiredBytes <= 0, or 0 initially before manual selection.
func (v VacateSpaceView) InitialProgressPercent() int {
	if v.RequiredBytes <= 0 {
		return 100
	}
	return 0
}

// FormatBytes formats a byte count into a concise human-readable unit (B, KB, MB, GB).
func FormatBytes(b int64) string {
	if b < 0 {
		b = 0
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
