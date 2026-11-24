package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SnapshotVersion identifies the JSON backup format produced by ExportSnapshot.
const SnapshotVersion = 1

// Snapshot holds a portable offline JSON backup of all persistent Res Imperii records.
type Snapshot struct {
	Version    int             `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Journals   []Journal       `json:"journals"`
	Books      []Book          `json:"books"`
	Passkeys   []PasskeyRecord `json:"passkeys"`
}

// ExportSnapshot reads all journals, books, and passkeys directly from Turso without
// modifying any data, producing a consistent offline backup snapshot.
func (s *Store) ExportSnapshot(ctx context.Context) (*Snapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database store is not initialized")
	}

	snap := &Snapshot{
		Version:    SnapshotVersion,
		ExportedAt: time.Now().UTC(),
		Journals:   []Journal{},
		Books:      []Book{},
		Passkeys:   []PasskeyRecord{},
	}

	// 1. Export journals
	jRows, err := s.db.QueryContext(ctx,
		`SELECT `+journalColumns+` FROM journals ORDER BY id ASC;`,
	)
	if err != nil {
		return nil, fmt.Errorf("export journals: %w", err)
	}
	for jRows.Next() {
		j, err := scanJournal(jRows)
		if err != nil {
			jRows.Close()
			return nil, fmt.Errorf("scan journal: %w", err)
		}
		j.CreatedAt = j.CreatedAt.UTC()
		snap.Journals = append(snap.Journals, j)
	}
	if err := jRows.Err(); err != nil {
		jRows.Close()
		return nil, fmt.Errorf("iterate journals: %w", err)
	}
	jRows.Close()

	// 2. Export books
	bRows, err := s.db.QueryContext(ctx,
		`SELECT slug, title, body, updated_at FROM books ORDER BY slug ASC;`,
	)
	if err != nil {
		return nil, fmt.Errorf("export books: %w", err)
	}
	for bRows.Next() {
		var b Book
		var updatedMs int64
		if err := bRows.Scan(&b.Slug, &b.Title, &b.Body, &updatedMs); err != nil {
			bRows.Close()
			return nil, fmt.Errorf("scan book: %w", err)
		}
		b.UpdatedAt = time.UnixMilli(updatedMs).UTC()
		snap.Books = append(snap.Books, b)
	}
	if err := bRows.Err(); err != nil {
		bRows.Close()
		return nil, fmt.Errorf("iterate books: %w", err)
	}
	bRows.Close()

	// 3. Export passkey metadata & credentials
	pRows, err := s.db.QueryContext(ctx,
		`SELECT id, rp_id, name, credential_json, created_at, last_used_at FROM passkeys ORDER BY created_at ASC, id ASC;`,
	)
	if err != nil {
		return nil, fmt.Errorf("export passkeys: %w", err)
	}
	for pRows.Next() {
		var rec PasskeyRecord
		var createdMs, lastUsedMs int64
		if err := pRows.Scan(&rec.ID, &rec.RPID, &rec.Name, &rec.CredentialJSON, &createdMs, &lastUsedMs); err != nil {
			pRows.Close()
			return nil, fmt.Errorf("scan passkey: %w", err)
		}
		rec.CreatedAt = time.UnixMilli(createdMs).UTC()
		if lastUsedMs > 0 {
			rec.LastUsedAt = time.UnixMilli(lastUsedMs).UTC()
		}
		snap.Passkeys = append(snap.Passkeys, rec)
	}
	if err := pRows.Err(); err != nil {
		pRows.Close()
		return nil, fmt.Errorf("iterate passkeys: %w", err)
	}
	pRows.Close()

	return snap, nil
}

// RestoreSnapshot validates and upserts journals, books, and passkeys from snap inside
// a single database transaction serialized by writeMu.
func (s *Store) RestoreSnapshot(ctx context.Context, snap *Snapshot) error {
	if s == nil || s.db == nil {
		return errors.New("database store is not initialized")
	}
	if snap == nil {
		return errors.New("snapshot is nil")
	}
	if snap.Version <= 0 {
		return fmt.Errorf("unsupported snapshot version: %d", snap.Version)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	const upsertJournalSQL = `
INSERT INTO journals (id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published, passage)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  created_at = excluded.created_at,
  media_url = excluded.media_url,
  tag1 = excluded.tag1,
  tag2 = excluded.tag2,
  tag3 = excluded.tag3,
  body = excluded.body,
  bookmarked = excluded.bookmarked,
  published = excluded.published,
  passage = excluded.passage;`

	for _, j := range snap.Journals {
		if j.ID <= 0 {
			return fmt.Errorf("invalid journal id %d in snapshot", j.ID)
		}
		entry, err := ParseValidatedEntry(JournalInput{
			MediaURL:    j.MediaURL,
			Tags:        j.Tags(),
			Body:        j.Body,
			Bookmarked:  j.Bookmarked,
			Published:   j.Published,
			PassageJSON: j.PassageJSON,
		})
		if err != nil {
			return fmt.Errorf("invalid journal %d in snapshot: %w", j.ID, err)
		}
		createdAtMs := j.CreatedAt.UTC().UnixMilli()
		if j.CreatedAt.IsZero() {
			createdAtMs = j.ID
		}
		if _, err := tx.ExecContext(ctx, upsertJournalSQL,
			j.ID,
			createdAtMs,
			entry.MediaURL(),
			nullableTag(entry.TagAt(0)),
			nullableTag(entry.TagAt(1)),
			nullableTag(entry.TagAt(2)),
			entry.Body(),
			boolToInt(entry.Bookmarked()),
			boolToInt(entry.Published()),
			nullablePassage(entry.PassageJSON()),
		); err != nil {
			return fmt.Errorf("restore journal %d: %w", j.ID, err)
		}
	}

	const upsertBookSQL = `
INSERT INTO books (slug, title, body, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
  title = excluded.title,
  body = excluded.body,
  updated_at = excluded.updated_at;`

	for _, b := range snap.Books {
		title, ok := BookTitle(b.Slug)
		if !ok {
			return fmt.Errorf("invalid book slug %q in snapshot", b.Slug)
		}
		cleanBody, err := ValidateBookBody(b.Body)
		if err != nil {
			return fmt.Errorf("invalid book %q in snapshot: %w", b.Slug, err)
		}
		updatedAtMs := b.UpdatedAt.UTC().UnixMilli()
		if b.UpdatedAt.IsZero() {
			updatedAtMs = time.Now().UTC().UnixMilli()
		}
		if _, err := tx.ExecContext(ctx, upsertBookSQL, b.Slug, title, cleanBody, updatedAtMs); err != nil {
			return fmt.Errorf("restore book %s: %w", b.Slug, err)
		}
	}

	const upsertPasskeySQL = `
INSERT INTO passkeys (id, rp_id, name, credential_json, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  rp_id = excluded.rp_id,
  name = excluded.name,
  credential_json = excluded.credential_json,
  created_at = excluded.created_at,
  last_used_at = excluded.last_used_at;`

	for _, pk := range snap.Passkeys {
		id := strings.TrimSpace(pk.ID)
		rpID := strings.TrimSpace(pk.RPID)
		if id == "" || rpID == "" || strings.TrimSpace(pk.CredentialJSON) == "" {
			continue
		}
		name := strings.TrimSpace(pk.Name)
		if name == "" {
			name = "Passkey"
		}
		createdAtMs := pk.CreatedAt.UTC().UnixMilli()
		if pk.CreatedAt.IsZero() {
			createdAtMs = time.Now().UTC().UnixMilli()
		}
		var lastUsedMs int64
		if !pk.LastUsedAt.IsZero() {
			lastUsedMs = pk.LastUsedAt.UTC().UnixMilli()
		}
		if _, err := tx.ExecContext(ctx, upsertPasskeySQL,
			id, rpID, name, pk.CredentialJSON, createdAtMs, lastUsedMs,
		); err != nil {
			return fmt.Errorf("restore passkey %s: %w", id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.invalidateCache()
	s.bookCacheMu.Lock()
	s.cachedBooks = nil
	s.bookCacheAt = nil
	s.bookCacheMu.Unlock()

	return nil
}
