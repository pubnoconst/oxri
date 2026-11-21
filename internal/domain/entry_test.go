package domain_test

import (
	"testing"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestNewValidatedEntry(t *testing.T) {
	path, err := domain.ParseSafePath("entry-01")
	if err != nil {
		t.Fatalf("ParseSafePath failed: %v", err)
	}

	tags, err := domain.ParseTagSet([]string{"philosophy", "reflections"})
	if err != nil {
		t.Fatalf("ParseTagSet failed: %v", err)
	}

	body, err := domain.ParseProse("Today we seek refuge in simplicity.", 1000)
	if err != nil {
		t.Fatalf("ParseProse failed: %v", err)
	}

	media1, err := domain.ParseMediaURL("https://cdn.oxri.org/statue.jpg")
	if err != nil {
		t.Fatalf("ParseMediaURL failed: %v", err)
	}

	createdAt := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	entry, err := domain.NewValidatedEntry(path, tags, body, []domain.MediaURL{media1}, createdAt)
	if err != nil {
		t.Fatalf("NewValidatedEntry failed: %v", err)
	}

	if entry.Path().String() != "entry-01" {
		t.Errorf("entry.Path() = %q, want 'entry-01'", entry.Path().String())
	}
	if entry.Tags().Len() != 2 {
		t.Errorf("entry.Tags().Len() = %d, want 2", entry.Tags().Len())
	}
	if entry.Body().String() != "Today we seek refuge in simplicity." {
		t.Errorf("entry.Body() mismatch")
	}
	if len(entry.Media()) != 1 {
		t.Errorf("entry.Media() len = %d, want 1", len(entry.Media()))
	}
	if !entry.CreatedAt().Equal(createdAt) {
		t.Errorf("entry.CreatedAt() = %v, want %v", entry.CreatedAt(), createdAt)
	}

	// Reject uninitialized SafePath
	_, err = domain.NewValidatedEntry(domain.SafePath{}, tags, body, nil, createdAt)
	if err != domain.ErrZeroPath {
		t.Errorf("Expected ErrZeroPath, got %v", err)
	}

	// Reject uninitialized Prose
	_, err = domain.NewValidatedEntry(path, tags, domain.Prose{}, nil, createdAt)
	if err != domain.ErrZeroBody {
		t.Errorf("Expected ErrZeroBody, got %v", err)
	}
}
