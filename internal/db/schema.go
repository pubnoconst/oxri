package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const journalsTableDDL = `
CREATE TABLE IF NOT EXISTS %s (
  id INTEGER PRIMARY KEY,
  created_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000),
  media_url TEXT NOT NULL CHECK(length(media_url) <= 128 AND length(media_url) > 0),
  tag1 TEXT CHECK(tag1 IS NULL OR (length(tag1) >= 1 AND length(tag1) <= 8 AND tag1 = upper(substr(tag1, 1, 1)) || lower(substr(tag1, 2)))),
  tag2 TEXT CHECK(tag2 IS NULL OR (length(tag2) >= 1 AND length(tag2) <= 8 AND tag2 = upper(substr(tag2, 1, 1)) || lower(substr(tag2, 2)))),
  tag3 TEXT CHECK(tag3 IS NULL OR (length(tag3) >= 1 AND length(tag3) <= 8 AND tag3 = upper(substr(tag3, 1, 1)) || lower(substr(tag3, 2)))),
  body TEXT NOT NULL,
  bookmarked INTEGER NOT NULL DEFAULT 0 CHECK(bookmarked IN (0,1)),
  published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1)),
  passage TEXT CHECK(passage IS NULL OR length(passage) <= 12288),
  CHECK(
    (tag1 IS NULL OR tag2 IS NULL OR tag1 != tag2) AND
    (tag1 IS NULL OR tag3 IS NULL OR tag1 != tag3) AND
    (tag2 IS NULL OR tag3 IS NULL OR tag2 != tag3)
  )
);`

const schemaMigrationsTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000)
);`

const passkeysTableDDL = `
CREATE TABLE IF NOT EXISTS passkeys (
  id TEXT PRIMARY KEY,
  rp_id TEXT NOT NULL,
  name TEXT NOT NULL,
  credential_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  last_used_at INTEGER NOT NULL
);`

const passkeysIndexDDL = `
CREATE INDEX IF NOT EXISTS idx_passkeys_rp_id ON passkeys(rp_id);`

const booksTableDDL = `
CREATE TABLE IF NOT EXISTS books (
  slug TEXT PRIMARY KEY CHECK(slug IN ('book-i', 'book-ii', 'book-iii')),
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  updated_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000)
);`

const authStateTableDDL = `
CREATE TABLE IF NOT EXISTS auth_state (
  key TEXT PRIMARY KEY,
  value_int INTEGER NOT NULL,
  updated_at INTEGER NOT NULL DEFAULT (unixepoch() * 1000)
);`

// SchemaVersion returns the highest applied migration version recorded in schema_migrations,
// or 0 if no migrations have been recorded yet.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	if _, err := s.db.ExecContext(ctx, schemaMigrationsTableDDL); err != nil {
		return 0, err
	}
	var currentVersion int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations;`).Scan(&currentVersion); err != nil {
		return 0, err
	}
	return currentVersion, nil
}

// EnsureSchema creates the journals table if missing and upgrades older schemas
// inside an explicit database transaction, recording the applied schema version in
// schema_migrations so table rebuilds are atomic and version-tracked.
func (s *Store) EnsureSchema(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	currentVersion, err := s.SchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to query schema version: %w", err)
	}
	if currentVersion >= CurrentSchemaVersion {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin schema migration transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	createSQL := fmt.Sprintf(journalsTableDDL, "journals")
	if _, err := tx.ExecContext(ctx, createSQL); err != nil {
		return err
	}

	// Migration v1: Ensure `published` column exists via PRAGMA table_info(journals)
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(journals);`)
	if err != nil {
		return err
	}
	hasPublished := false
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "published" {
			hasPublished = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if !hasPublished {
		const alterSQL = `ALTER TABLE journals ADD COLUMN published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1));`
		if _, err := tx.ExecContext(ctx, alterSQL); err != nil {
			return fmt.Errorf("failed to add published column: %w", err)
		}
	}

	// Migration v2 & v5: Ensure the table DDL enforces Title Case, per-entry tag uniqueness, and max 8-char tags
	var tableSQL string
	err = tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'journals';`).Scan(&tableSQL)
	if err != nil {
		return err
	}

	if !strings.Contains(tableSQL, "tag1 != tag2") || !strings.Contains(tableSQL, "length(tag1) <= 8") {
		if err := rebuildJournalsWithTagConstraintsTx(ctx, tx); err != nil {
			return fmt.Errorf("failed to migrate journals tag constraints: %w", err)
		}
	}

	// Migration v3: Create passkeys table for FIDO2 / WebAuthn credentials keyed by rp_id
	if _, err := tx.ExecContext(ctx, passkeysTableDDL); err != nil {
		return fmt.Errorf("failed to create passkeys table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, passkeysIndexDDL); err != nil {
		return fmt.Errorf("failed to create passkeys index: %w", err)
	}

	// Migration v4: Create books table for Book I, II, III markdown content
	if _, err := tx.ExecContext(ctx, booksTableDDL); err != nil {
		return fmt.Errorf("failed to create books table: %w", err)
	}
	for _, seed := range defaultBooksSeed() {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO books (slug, title, body, updated_at) VALUES (?, ?, ?, ?);`,
			seed.Slug, seed.Title, seed.Body, seed.UpdatedAt.UnixMilli(),
		); err != nil {
			return fmt.Errorf("failed to seed book %s: %w", seed.Slug, err)
		}
	}

	// Migration v6: Create auth_state table for restart-safe TOTP replay watermarks
	if _, err := tx.ExecContext(ctx, authStateTableDDL); err != nil {
		return fmt.Errorf("failed to create auth_state table: %w", err)
	}

	// Migration v7: Create tweet_snapshots, the cache behind the site's own tweet box
	if _, err := tx.ExecContext(ctx, tweetSnapshotsTableDDL); err != nil {
		return fmt.Errorf("failed to create tweet_snapshots table: %w", err)
	}

	// Migration v8: the Bible passage an entry quotes (reference and plain text)
	if has, err := hasColumnTx(ctx, tx, "journals", "passage"); err != nil {
		return err
	} else if !has {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE journals ADD COLUMN passage TEXT CHECK(passage IS NULL OR length(passage) <= 12288);`); err != nil {
			return fmt.Errorf("failed to add journals.passage: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO schema_migrations (version) VALUES (?);`, CurrentSchemaVersion); err != nil {
		return fmt.Errorf("failed to record schema version %d: %w", CurrentSchemaVersion, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit schema migration transaction: %w", err)
	}
	s.invalidateCache()

	return nil
}

func rebuildJournalsWithTagConstraintsTx(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS journals_new;`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(journalsTableDDL, "journals_new")); err != nil {
		return err
	}

	// Copy rows while normalizing tags to Title Case and clearing any duplicate tag slots
	const copySQL = `
INSERT INTO journals_new (id, created_at, media_url, tag1, tag2, tag3, body, bookmarked, published)
SELECT
  id,
  created_at,
  media_url,
  CASE
    WHEN tag1 IS NULL OR trim(tag1) = '' THEN NULL
    ELSE upper(substr(trim(tag1), 1, 1)) || lower(substr(trim(tag1), 2))
  END,
  CASE
    WHEN tag2 IS NULL OR trim(tag2) = '' THEN NULL
    WHEN (upper(substr(trim(tag2), 1, 1)) || lower(substr(trim(tag2), 2))) =
         (upper(substr(trim(COALESCE(tag1, '')), 1, 1)) || lower(substr(trim(COALESCE(tag1, '')), 2))) THEN NULL
    ELSE upper(substr(trim(tag2), 1, 1)) || lower(substr(trim(tag2), 2))
  END,
  CASE
    WHEN tag3 IS NULL OR trim(tag3) = '' THEN NULL
    WHEN (upper(substr(trim(tag3), 1, 1)) || lower(substr(trim(tag3), 2))) IN (
      upper(substr(trim(COALESCE(tag1, '')), 1, 1)) || lower(substr(trim(COALESCE(tag1, '')), 2)),
      upper(substr(trim(COALESCE(tag2, '')), 1, 1)) || lower(substr(trim(COALESCE(tag2, '')), 2))
    ) THEN NULL
    ELSE upper(substr(trim(tag3), 1, 1)) || lower(substr(trim(tag3), 2))
  END,
  body,
  bookmarked,
  published
FROM journals;`
	if _, err := tx.ExecContext(ctx, copySQL); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE journals;`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE journals_new RENAME TO journals;`); err != nil {
		return err
	}
	return nil
}

// hasColumnTx reports whether table has a column named col.
func hasColumnTx(ctx context.Context, tx *sql.Tx, table, col string) (bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM pragma_table_info(?);`, table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, rows.Err()
}
