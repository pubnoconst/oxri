package db

import (
	"strings"

	"github.com/oxri/oxri-go/internal/domain"
)

// EstimateInputBytes is the byte footprint a JournalInput should have once
// stored, computed independently of the quota code so the tests can check
// that code against it.
func EstimateInputBytes(in JournalInput) int64 {
	total := RowOverheadBytes + int64(len(strings.TrimSpace(in.MediaURL))) + int64(len(in.Body))
	for i, tag := range in.Tags {
		if i >= domain.MaxJournalTagCount {
			break
		}
		total += int64(len(FormatTagTitleCase(tag)))
	}
	return total
}
