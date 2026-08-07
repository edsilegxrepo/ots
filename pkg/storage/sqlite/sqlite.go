// Package sqlite implements the OTS storage.Storage interface backed by a 100% pure Go, CGO-free SQLite database.
//
// Objectives:
// - Provide lightweight, zero-dependency embedded file or in-memory persistence for secrets.
// - Ensure high-concurrency safety using WAL journal mode, busy timeouts, and SQL transactions.
// - Automatically purge expired secrets via a background periodic cleanup ticker.
//
// Core Components:
// - Storage: Encapsulating database handle (*sql.DB), cleanup ticker stop channel, and close sync.Once.
// - New: Parses connection URIs ("sqlite:///path/to/ots.db" or "sqlite://:memory:"), initializes WAL mode, and creates table schemas.
// - Create, ReadAndDestroy, Delete, CleanupExpired: Perform atomic CRUD operations and read-and-destroy one-time access semantics.
//
// Data Flow:
// 1. New() -> Open modernc.org/sqlite DB -> Enable WAL & busy_timeout -> CREATE TABLE IF NOT EXISTS secrets.
// 2. Create() -> Insert secret with calculated expires_at and requested_reads count.
// 3. ReadAndDestroy() -> BeginTx -> SELECT secret & remaining reads -> Decrement reads or DELETE if final read -> Commit.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	_ "modernc.org/sqlite"

	"github.com/edsilegxrepo/ots/pkg/storage"
)

// Storage implements the storage.Storage interface backed by SQLite
type Storage struct {
	db        *sql.DB
	closeOnce sync.Once
	stopChan  chan struct{}
}

// New initializes a pure Go SQLite storage instance (e.g. "sqlite:///path/to/ots.db" or "sqlite://:memory:")
func New(connStr string) (*Storage, error) {
	dbPath := ":memory:"
	if connStr != "" {
		trimmed := strings.TrimPrefix(connStr, "sqlite://")
		trimmed = strings.TrimPrefix(trimmed, "sqlite:")
		if trimmed != "" {
			u, err := url.Parse(trimmed)
			if err == nil && u.Path != "" {
				dbPath = u.Path
			} else {
				dbPath = trimmed
			}
		}
	}

	if dbPath != ":memory:" {
		dbPath = filepath.Clean(dbPath)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// Apply WAL mode and busy timeout for concurrent safety
	_, _ = db.Exec("PRAGMA journal_mode=WAL;")
	_, _ = db.Exec("PRAGMA busy_timeout=5000;")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL;")

	// Initialize table schema
	schema := `
	CREATE TABLE IF NOT EXISTS secrets (
		id TEXT PRIMARY KEY,
		secret TEXT NOT NULL,
		reads INTEGER NOT NULL,
		expires_at INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_expires_at ON secrets(expires_at);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite init schema: %w", err)
	}

	s := &Storage{
		db:       db,
		stopChan: make(chan struct{}),
	}

	// Start background cleanup ticker
	go s.startExpirationTicker()

	return s, nil
}

// Close closes the SQLite database connection and stops the background ticker
func (s *Storage) Close() error {
	s.closeOnce.Do(func() {
		close(s.stopChan)
	})
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("sqlite close db: %w", err)
	}
	return nil
}

func (s *Storage) startExpirationTicker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			now := time.Now().Unix()
			_, _ = s.db.Exec("DELETE FROM secrets WHERE expires_at > 0 AND expires_at < ?;", now)
		}
	}
}

// Count returns the total number of unexpired secrets
func (s *Storage) Count() (int64, error) {
	now := time.Now().Unix()
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM secrets WHERE expires_at = 0 OR expires_at >= ?;", now).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite count query: %w", err)
	}
	return count, nil
}

// Create inserts a new secret with total allowed reads and returns its ID
func (s *Storage) Create(secret string, expireIn time.Duration, reads int) (string, error) {
	id := uuid.Must(uuid.NewV4()).String()
	if reads < 1 {
		reads = 1
	}

	var expiresAt int64
	if expireIn > 0 {
		expiresAt = time.Now().Add(expireIn).Unix()
	}

	query := "INSERT INTO secrets (id, secret, reads, expires_at) VALUES (?, ?, ?, ?);"
	_, err := s.db.Exec(query, id, secret, reads, expiresAt)
	if err != nil {
		return "", fmt.Errorf("sqlite insert secret: %w", err)
	}

	return id, nil
}

// ReadAndDestroy returns secret content, decrements remaining reads, and destroys entry when reads <= 0
func (s *Storage) ReadAndDestroy(id string) (string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, fmt.Errorf("sqlite begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	var (
		secret    string
		reads     int
		expiresAt int64
	)

	query := "SELECT secret, reads, expires_at FROM secrets WHERE id = ?;"
	err = tx.QueryRowContext(ctx, query, id).Scan(&secret, &reads, &expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", 0, storage.ErrSecretNotFound
		}
		return "", 0, fmt.Errorf("sqlite scan row: %w", err)
	}

	// Check expiration
	if expiresAt > 0 && expiresAt <= now {
		_, _ = tx.ExecContext(ctx, "DELETE FROM secrets WHERE id = ?;", id)
		_ = tx.Commit()
		return "", 0, storage.ErrSecretNotFound
	}

	readsRemaining := reads - 1

	if readsRemaining <= 0 {
		_, err = tx.ExecContext(ctx, "DELETE FROM secrets WHERE id = ?;", id)
		if err != nil {
			return "", 0, fmt.Errorf("sqlite delete secret: %w", err)
		}
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE secrets SET reads = ? WHERE id = ?;", readsRemaining, id)
		if err != nil {
			return "", 0, fmt.Errorf("sqlite update remaining reads: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", 0, fmt.Errorf("sqlite commit transaction: %w", err)
	}

	return secret, readsRemaining, nil
}
