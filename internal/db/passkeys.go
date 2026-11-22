package db

import (
	"context"
	"errors"
	"strings"
	"time"
)

// PasskeyRecord represents a persisted FIDO2 / WebAuthn credential bound to a specific RP ID.
type PasskeyRecord struct {
	ID             string
	RPID           string
	Name           string
	CredentialJSON string
	CreatedAt      time.Time
	LastUsedAt     time.Time
}

// ListPasskeys returns all enrolled passkeys for rpID ordered newest first.
func (s *Store) ListPasskeys(ctx context.Context, rpID string) ([]PasskeyRecord, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database store is not initialized")
	}
	const q = `SELECT id, rp_id, name, credential_json, created_at, last_used_at FROM passkeys WHERE rp_id = ? ORDER BY created_at DESC;`
	rows, err := s.db.QueryContext(ctx, q, strings.TrimSpace(rpID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PasskeyRecord
	for rows.Next() {
		var rec PasskeyRecord
		var createdMs, lastUsedMs int64
		if err := rows.Scan(&rec.ID, &rec.RPID, &rec.Name, &rec.CredentialJSON, &createdMs, &lastUsedMs); err != nil {
			return nil, err
		}
		rec.CreatedAt = time.UnixMilli(createdMs).UTC()
		if lastUsedMs > 0 {
			rec.LastUsedAt = time.UnixMilli(lastUsedMs).UTC()
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// CountPasskeys returns the number of enrolled passkeys for rpID.
func (s *Store) CountPasskeys(ctx context.Context, rpID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM passkeys WHERE rp_id = ?;`, strings.TrimSpace(rpID)).Scan(&count)
	return count, err
}

// SavePasskey inserts a new WebAuthn passkey record into Turso.
func (s *Store) SavePasskey(ctx context.Context, rec PasskeyRecord) error {
	if s == nil || s.db == nil {
		return errors.New("database store is not initialized")
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	var lastUsedMs int64
	if !rec.LastUsedAt.IsZero() {
		lastUsedMs = rec.LastUsedAt.UnixMilli()
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	const q = `INSERT INTO passkeys (id, rp_id, name, credential_json, created_at, last_used_at) VALUES (?, ?, ?, ?, ?, ?);`
	_, err := s.db.ExecContext(ctx, q, rec.ID, strings.TrimSpace(rec.RPID), strings.TrimSpace(rec.Name), rec.CredentialJSON, rec.CreatedAt.UnixMilli(), lastUsedMs)
	return err
}

// UpdatePasskeyCredential updates the serialized WebAuthn credential (sign count / backup flags)
// and last_used_at timestamp after a successful assertion.
func (s *Store) UpdatePasskeyCredential(ctx context.Context, id, rpID, credJSON string, lastUsedAt time.Time) error {
	if s == nil || s.db == nil {
		return errors.New("database store is not initialized")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	const q = `UPDATE passkeys SET credential_json = ?, last_used_at = ? WHERE id = ? AND rp_id = ?;`
	_, err := s.db.ExecContext(ctx, q, credJSON, lastUsedAt.UTC().UnixMilli(), id, strings.TrimSpace(rpID))
	return err
}

// DeletePasskey removes an enrolled passkey for rpID.
func (s *Store) DeletePasskey(ctx context.Context, id, rpID string) error {
	if s == nil || s.db == nil {
		return errors.New("database store is not initialized")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	const q = `DELETE FROM passkeys WHERE id = ? AND rp_id = ?;`
	_, err := s.db.ExecContext(ctx, q, id, strings.TrimSpace(rpID))
	return err
}
