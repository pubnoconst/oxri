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

// AdminCap is a capability token representing verified administrative authority.
// Functions requiring administrative authority must mandate an AdminCap parameter.
//
// A *valid* AdminCap can only be minted inside internal/domain, via the one-time
// CapMinter claimed by internal/auth after it verifies a session. Go cannot
// forbid the zero value (AdminCap{} compiles anywhere), so the zero value means
// "no authority": administrative store writes reject it at runtime (db.ErrUnauthorized),
// and TestPrivilegedCallSites fails the build if the literal appears outside the
// code that reports authentication failures.
type AdminCap struct {
	issuedAt time.Time
	_        [0]func() // prevents comparison (==)
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
