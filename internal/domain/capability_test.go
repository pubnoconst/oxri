package domain_test

import (
	"testing"

	"github.com/oxri/oxri-go/internal/domain"
)

func TestAdminCapMinter(t *testing.T) {
	minter := domain.SetupCapMinter()
	if minter == nil {
		t.Fatalf("SetupCapMinter() first call returned nil")
	}

	// Second call must return nil to prevent unauthorized callers from minting
	secondMinter := domain.SetupCapMinter()
	if secondMinter != nil {
		t.Errorf("SetupCapMinter() second call expected nil, got non-nil")
	}

	cap := minter()
	if cap.IsZero() {
		t.Errorf("minter() produced a zero capability token")
	}
	if cap.IssuedAt().IsZero() {
		t.Errorf("cap.IssuedAt() is zero")
	}
}
