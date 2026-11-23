// Package db provides remote Turso/libSQL persistence, transactional schema migrations,
// and atomic content-budget quota management for Res Imperii.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/oxri/oxri-go/internal/domain"
	"github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

const (
	MaxMediaURLLength    = domain.MaxJournalMediaURLLength
	MaxTagCount          = domain.MaxJournalTagCount
	MaxTagLength         = domain.MaxJournalTagLength
	CurrentSchemaVersion = 6
	readCacheTTL         = 2 * time.Minute
)

var (
	ErrNotFound     = errors.New("journal entry not found")
	ErrInvalidInput = domain.ErrInvalidJournalInput
)

// Store wraps SQL operations against a remote Turso / libSQL database.
// writeMu serializes transactional write operations (Create, Update, AutoVacateBytes, Delete)
// so concurrent goroutines cannot race past the content storage quota.
// cacheMu guards an in-memory read snapshot invalidated immediately on any write,
// eliminating cross-continent round-trip latency on catalog and entry reads.
type Store struct {
	db              *sql.DB
	writeMu         sync.Mutex
	idMu            sync.Mutex
	lastID          int64
	maxStorageBytes int64
	pendingSub      *PendingSubmission

	cacheMu       sync.RWMutex
	cachedEntries []Journal
	cacheValid    bool
	cacheLoadedAt time.Time

	bookCacheMu sync.RWMutex
	cachedBooks map[string]Book
	bookCacheAt map[string]time.Time
}

// Open connects to a remote Turso/libSQL URL (libsql:// or https://), verifies
// connectivity, runs schema migrations, and warms the in-memory read cache.
// Local file databases are disabled; only ":memory:" is accepted for unit testing.
func Open(ctx context.Context, dbURL, authToken string) (*Store, error) {
	dbURL = strings.TrimSpace(dbURL)
	if dbURL == "" {
		return nil, errors.New("TURSO_DATABASE_URL is required (local databases are disabled)")
	}
	if strings.HasPrefix(strings.ToLower(dbURL), "file:") {
		return nil, errors.New("local file databases are disabled; use a remote Turso libsql:// or https:// URL")
	}

	var sqlDB *sql.DB
	if dbURL == ":memory:" {
		db, err := sql.Open("sqlite", dbURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open in-memory test database: %w", err)
		}
		// Keep a single connection for in-memory SQLite so concurrent goroutines share the same memory DB
		db.SetMaxOpenConns(1)
		sqlDB = db
	} else {
		var opts []libsql.Option
		if authToken != "" {
			opts = append(opts, libsql.WithAuthToken(authToken))
		}
		connector, err := libsql.NewConnector(dbURL, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create libsql connector: %w", err)
		}
		sqlDB = sql.OpenDB(connector)
		sqlDB.SetMaxIdleConns(4)
		sqlDB.SetMaxOpenConns(8)
		sqlDB.SetConnMaxLifetime(30 * time.Minute)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	store := &Store{db: sqlDB}
	if err := store.EnsureSchema(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("schema initialization failed: %w", err)
	}

	// Warm the read cache asynchronously for remote databases so startup and initial /journals/content are fast
	if dbURL != ":memory:" {
		go func() {
			warmCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = store.allEntriesCached(warmCtx)
		}()
	}

	return store, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) invalidateCache() {
	s.cacheMu.Lock()
	s.cacheValid = false
	s.cachedEntries = nil
	s.cacheMu.Unlock()
}

func (s *Store) nextID() int64 {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	now := time.Now().UnixMilli()
	if now <= s.lastID {
		now = s.lastID + 1
	}
	s.lastID = now
	return now
}

type queryExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
