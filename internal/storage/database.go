// Package storage handles data persistence using SQLite
package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/adrg/xdg"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection at the specified path
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	return db, nil
}

// NewDefault creates a new database connection at the default XDG state location
func NewDefault() (*DB, error) {
	dbPath, err := xdg.StateFile("izrss/izrss.db")
	if err != nil {
		return nil, fmt.Errorf("getting state file path: %w", err)
	}
	return New(dbPath)
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

func (db *DB) createTables() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS post_read_status (
			uuid TEXT PRIMARY KEY,
			feed_url TEXT NOT NULL,
			read INTEGER NOT NULL DEFAULT 0
		);

		CREATE TABLE IF NOT EXISTS cache_metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS feed_cache (
			url TEXT PRIMARY KEY,
			content BLOB NOT NULL,
			fetched_at TEXT NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_feed_url ON post_read_status(feed_url);
	`)
	return err
}

// PostReadStatus represents a post's read status in the database
type PostReadStatus struct {
	UUID    string
	FeedURL string
	Read    bool
}

// SavePostReadStatus saves the read status for a single post
func (db *DB) SavePostReadStatus(uuid, feedURL string, read bool) error {
	readInt := 0
	if read {
		readInt = 1
	}

	_, err := db.conn.Exec(`
		INSERT INTO post_read_status (uuid, feed_url, read)
		VALUES (?, ?, ?)
		ON CONFLICT(uuid) DO UPDATE SET read = excluded.read
	`, uuid, feedURL, readInt)

	return err
}

// SavePostReadStatuses saves multiple read statuses in a single transaction
func (db *DB) SavePostReadStatuses(statuses []PostReadStatus) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
    err := tx.Rollback()
    if err != nil && err != sql.ErrTxDone {
        fmt.Printf("transaction rollback error: %v\n", err)
    }
  }()

	stmt, err := tx.Prepare(`
		INSERT INTO post_read_status (uuid, feed_url, read)
		VALUES (?, ?, ?)
		ON CONFLICT(uuid) DO UPDATE SET read = excluded.read
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, status := range statuses {
		readInt := 0
		if status.Read {
			readInt = 1
		}
		if _, err = stmt.Exec(status.UUID, status.FeedURL, readInt); err != nil {
			return fmt.Errorf("saving post %s: %w", status.UUID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// LoadPostReadStatuses returns a map of UUID to read status
func (db *DB) LoadPostReadStatuses() (map[string]bool, error) {
	rows, err := db.conn.Query(`SELECT uuid, read FROM post_read_status`)
	if err != nil {
		return nil, fmt.Errorf("querying read statuses: %w", err)
	}
	defer rows.Close()

	statuses := make(map[string]bool)
	for rows.Next() {
		var uuid string
		var read int
		if err := rows.Scan(&uuid, &read); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		statuses[uuid] = read == 1
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return statuses, nil
}

// GetCacheTime retrieves the last fetch time from the database
func (db *DB) GetCacheTime() (*time.Time, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM cache_metadata WHERE key = 'last_fetch_time'`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying cache time: %w", err)
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parsing cache time: %w", err)
	}

	return &t, nil
}

// SetCacheTime stores the current time as the last fetch time
func (db *DB) SetCacheTime() error {
	_, err := db.conn.Exec(`
		INSERT INTO cache_metadata (key, value)
		VALUES ('last_fetch_time', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, time.Now().Format(time.RFC3339))
	return err
}

// SaveFeedCache stores the fetched feed content in the database
func (db *DB) SaveFeedCache(url string, content []byte) error {
	_, err := db.conn.Exec(`
		INSERT INTO feed_cache (url, content, fetched_at)
		VALUES (?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET content = excluded.content, fetched_at = excluded.fetched_at
	`, url, content, time.Now().Format(time.RFC3339))
	return err
}

// LoadFeedCache retrieves cached feed content from the database
func (db *DB) LoadFeedCache(url string) ([]byte, error) {
	var content []byte
	err := db.conn.QueryRow(`SELECT content FROM feed_cache WHERE url = ?`, url).Scan(&content)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying feed cache: %w", err)
	}
	return content, nil
}

// ClearFeedCache removes all cached feed content
func (db *DB) ClearFeedCache() error {
	_, err := db.conn.Exec(`DELETE FROM feed_cache`)
	return err
}
