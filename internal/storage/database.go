// Package storage handles data persistence using SQLite
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/adrg/xdg"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection at the specified path. WAL mode and a
// busy timeout let the background refresh write without "database is locked".
func New(path string) (*DB, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Bound the pool and release idle connections so a refresh can't keep a
	// connection open per feed.
	conn.SetMaxOpenConns(8)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxIdleTime(time.Minute)

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		_ = conn.Close()
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

func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

func (db *DB) createTables() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS feed_cache (
			url TEXT PRIMARY KEY,
			content BLOB NOT NULL,
			fetched_at TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	return db.migrate()
}

// schemaVersion is the PRAGMA user_version of the current schema.
//
//	0: post_read_status keyed by uuid (the item GUID) alone, so GUID-less
//	   posts shared one row and a GUID in two feeds shared one read flag.
//	1: post_read_status keyed by (feed_url, key), key being rss.Post.Key().
const schemaVersion = 1

// migrate brings the schema up to schemaVersion. It is a single PRAGMA read
// when the database is already current.
func (db *DB) migrate() error {
	var v int
	if err := db.conn.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if v >= schemaVersion {
		return nil
	}

	ctx := context.Background()
	conn, err := db.conn.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	// IMMEDIATE takes the write lock up front, so two instances starting at
	// once migrate one after the other rather than racing.
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("beginning migration: %w", err)
	}
	if err := migrateV1(ctx, conn); err != nil {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		return fmt.Errorf("migrating schema to v1: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("committing migration: %w", err)
	}
	return nil
}

// migrateV1 rebuilds post_read_status keyed by (feed_url, key). A v0 row's
// uuid is a GUID, which is exactly Post.Key() for that post, and its feed_url
// is the feed it was first saved from, so rows carry over as they are. The
// empty-uuid row, shared by every GUID-less post, is meaningless and dropped:
// those posts start unread under their new link- or title-based keys. Must
// run inside a transaction.
func migrateV1(ctx context.Context, conn *sql.Conn) error {
	var v int
	if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	if v >= 1 { // another instance got here first
		return nil
	}
	var old int
	if err := conn.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'post_read_status'`).Scan(&old); err != nil {
		return err
	}
	stmts := []string{`
		CREATE TABLE post_read_status_v1 (
			feed_url TEXT NOT NULL,
			key TEXT NOT NULL,
			read INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (feed_url, key)
		)`}
	if old > 0 {
		stmts = append(stmts,
			`INSERT INTO post_read_status_v1 (feed_url, key, read)
				SELECT feed_url, uuid, read FROM post_read_status WHERE uuid <> ''`,
			`DROP TABLE post_read_status`, // and its idx_feed_url; the primary key covers feed_url
		)
	}
	stmts = append(stmts,
		`ALTER TABLE post_read_status_v1 RENAME TO post_read_status`,
		`PRAGMA user_version = 1`,
	)
	for _, q := range stmts {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// PostReadStatus is one post's read state; Key is rss.Post.Key().
type PostReadStatus struct {
	FeedURL string
	Key     string
	Read    bool
}

// upsertReadStatus saves one post's read status; go-sqlite3 stores bools as
// INTEGER 1 or 0.
const upsertReadStatus = `
	INSERT INTO post_read_status (feed_url, key, read)
	VALUES (?, ?, ?)
	ON CONFLICT(feed_url, key) DO UPDATE SET read = excluded.read
`

// SavePostReadStatuses saves read statuses in a single transaction
func (db *DB) SavePostReadStatuses(statuses []PostReadStatus) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		err := tx.Rollback()
		if err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("transaction rollback error: %v", err)
		}
	}()

	stmt, err := tx.Prepare(upsertReadStatus)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, status := range statuses {
		if _, err = stmt.Exec(status.FeedURL, status.Key, status.Read); err != nil {
			return fmt.Errorf("saving post %s: %w", status.Key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// LoadPostReadStatuses returns every saved read status
func (db *DB) LoadPostReadStatuses() ([]PostReadStatus, error) {
	rows, err := db.conn.Query(`SELECT feed_url, key, read FROM post_read_status`)
	if err != nil {
		return nil, fmt.Errorf("querying read statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var statuses []PostReadStatus
	for rows.Next() {
		var st PostReadStatus
		if err := rows.Scan(&st.FeedURL, &st.Key, &st.Read); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		statuses = append(statuses, st)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return statuses, nil
}

// LastFetched is when a feed was last fetched and cached, the zero time if
// none was.
func (db *DB) LastFetched() (time.Time, error) {
	rows, err := db.conn.Query(`SELECT fetched_at FROM feed_cache`)
	if err != nil {
		return time.Time{}, fmt.Errorf("querying fetch times: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var last time.Time
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return time.Time{}, fmt.Errorf("scanning row: %w", err)
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil && t.After(last) {
			last = t
		}
	}
	return last, rows.Err()
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
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying feed cache: %w", err)
	}
	return content, nil
}
