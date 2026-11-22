package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const authStateKeyLastTOTPStep = "last_totp_step"

// LoadLastTOTPStep returns the highest persisted accepted RFC 6238 TOTP time step,
// or -1 if no code has been recorded yet.
func (s *Store) LoadLastTOTPStep(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return -1, nil
	}
	var step int64
	err := s.db.QueryRowContext(ctx,
		`SELECT value_int FROM auth_state WHERE key = ?;`,
		authStateKeyLastTOTPStep,
	).Scan(&step)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return -1, nil
		}
		return -1, err
	}
	return step, nil
}

// SaveLastTOTPStep monotonically persists the highest accepted TOTP time step so
// process restarts within the same 30-second interval cannot accept a replayed code.
func (s *Store) SaveLastTOTPStep(ctx context.Context, step int64) error {
	if s == nil || s.db == nil {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	nowMs := time.Now().UTC().UnixMilli()
	const q = `
INSERT INTO auth_state (key, value_int, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
  value_int = CASE WHEN excluded.value_int > auth_state.value_int THEN excluded.value_int ELSE auth_state.value_int END,
  updated_at = excluded.updated_at;`
	_, err := s.db.ExecContext(ctx, q, authStateKeyLastTOTPStep, step, nowMs)
	return err
}
