package domain

import (
	"sync"
	"time"
)

var (
	capOnce   sync.Once
	minterRef CapMinter
)

// CapMinter is a function signature that mints an AdminCap.
// Only the subsystem that receives this during one-time startup (internal/auth)
// can ever mint AdminCap instances.
type CapMinter func() AdminCap

// AdminCap is a compile-time and runtime capability token representing verified administrative authority.
// It possesses unexported fields and cannot be constructed outside internal/domain.
// Functions requiring administrative authority must mandate an AdminCap parameter.
type AdminCap struct {
	issuedAt time.Time
	_        [0]func() // prevents struct comparison and literal construction
}

// SetupCapMinter returns the unique CapMinter function.
// It can only be called once during the lifetime of the process.
// Any subsequent call returns nil, preventing unauthorized subsystems from acquiring minting capabilities.
func SetupCapMinter() CapMinter {
	var m CapMinter
	capOnce.Do(func() {
		minterRef = func() AdminCap {
			return AdminCap{
				issuedAt: time.Now().UTC(),
			}
		}
		m = minterRef
	})
	return m
}

// IssuedAt returns when this capability token was minted.
func (c AdminCap) IssuedAt() time.Time {
	return c.issuedAt
}

// IsZero reports whether the capability token is uninitialized.
func (c AdminCap) IsZero() bool {
	return c.issuedAt.IsZero()
}
