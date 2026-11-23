package db

import (
	"errors"

	"github.com/oxri/oxri-go/internal/domain"
)

// ErrUnauthorized is returned by every administrative write when it is handed
// a zero (forged or uninitialised) domain.AdminCap.
var ErrUnauthorized = errors.New("db: administrative capability required")

// requireCap is the first statement of every administrative write.
//
// The type system makes the capability impossible to forget: an
// administrative write cannot be called without a domain.AdminCap in hand,
// and only internal/auth can mint a real one (after verifying a session).
// Go cannot forbid the zero value domain.AdminCap{}, so this runtime check
// rejects it, and TestPrivilegedCallSites (routes) fails the build if a zero
// literal appears anywhere it is not allowed.
//
// Deliberately uncapped writes (no session can exist when they run):
//   - SaveLastTOTPStep, UpdatePasskeyCredential: sign-in bookkeeping, made
//     while authenticating, before any session exists.
//   - RestoreSnapshot: offline operator tool (cmd/backup) holding raw database
//     credentials; pinned to that call site by TestPrivilegedCallSites.
//   - SetMaxStorageBytes: process configuration at startup.
func requireCap(cap domain.AdminCap) error {
	if cap.IsZero() {
		return ErrUnauthorized
	}
	return nil
}
