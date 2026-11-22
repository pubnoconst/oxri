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
  tag1 TEXT CHECK(tag1 IS NULL OR (length(tag1) >= 1 AND length(tag1) <= 4 AND tag1 = upper(substr(tag1, 1, 1)) || lower(substr(tag1, 2)))),
  tag2 TEXT CHECK(tag2 IS NULL OR (length(tag2) >= 1 AND length(tag2) <= 4 AND tag2 = upper(substr(tag2, 1, 1)) || lower(substr(tag2, 2)))),
  tag3 TEXT CHECK(tag3 IS NULL OR (length(tag3) >= 1 AND length(tag3) <= 4 AND tag3 = upper(substr(tag3, 1, 1)) || lower(substr(tag3, 2)))),
  body TEXT NOT NULL,
  bookmarked INTEGER NOT NULL DEFAULT 0 CHECK(bookmarked IN (0,1)),
  published INTEGER NOT NULL DEFAULT 1 CHECK(published IN (0,1)),
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

	// Migration v2: Ensure the table DDL enforces Title Case and per-entry tag uniqueness
	var tableSQL string
	err = tx.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'journals';`).Scan(&tableSQL)
	if err != nil {
		return err
	}

	if !strings.Contains(tableSQL, "tag1 != tag2") {
		if err := rebuildJournalsWithTagConstraintsTx(ctx, tx); err != nil {
			return fmt.Errorf("failed to migrate journals tag constraints: %w", err)
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
